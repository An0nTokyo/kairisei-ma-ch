package cnbootstrap

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"kairisei.local/server/internal/accounthttp"
	"kairisei.local/server/internal/accountstore"
	adminapi "kairisei.local/server/internal/admin"
	"kairisei.local/server/internal/game"
	"kairisei.local/server/internal/gamestate"
	"kairisei.local/server/internal/httpapi"
	"kairisei.local/server/internal/masterdata"
	"kairisei.local/server/internal/protocol"
)

const (
	cn602CatalogVersion = 790
	cn602MenuBundle     = "main_c/scenes/scene_Menu.dat"
	cn602LocalSession   = "local-cn-session-key"
	// UI availability bits are independent of the fixed HTTP route set.
	cn602LocalModuleSwitchState = int64(protocol.ModuleEverydayTask | protocol.ModuleActivity |
		protocol.ModuleRecommendDeck | protocol.ModuleStrategyButton | protocol.ModulePVP |
		protocol.ModuleFailureAdvise | protocol.ModuleCopyCard | protocol.ModuleChangeCloth |
		protocol.ModuleChangeModel | protocol.ModuleReturnToDungeon)
)

var cn602ScrambleKey = [...]byte{0x01, 0xcd, 0x45, 0x89, 0x67, 0xab, 0x23, 0xef}

// New assembles the CN protocol adapter, account runtime and resource service.
// The returned handler implements io.Closer; its owner must close it after all
// HTTP/Admin/BattleSv work has drained, including on listener setup failure.
func New(config Config) (http.Handler, error) {
	app, err := assembleApplication(config)
	if err != nil {
		return nil, err
	}
	handler, err := app.handler()
	if err == nil {
		err = app.accounts.InvalidateSessions()
	}
	if err != nil {
		_ = app.accounts.Database().Close()
		return nil, err
	}
	return handler, err
}

func assembleApplication(config Config) (*application, error) {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	cdn, cdnErr := config.CDN.normalized()
	if cdnErr != nil {
		return nil, cdnErr
	}
	config.CDN = cdn
	if config.Multiplayer == nil || config.Network.BattleSV.Host == "" || config.Network.BattleSV.Port == 0 {
		return nil, errors.New("local multiplayer runtime is required")
	}
	if config.Persistence.RequestLog == "" {
		return nil, errors.New("request log path is required")
	}
	if config.Network.AdvertiseHost == "" {
		return nil, errors.New("advertise host is required")
	}
	if config.Network.HTTPPort < 1 || config.Network.HTTPPort > 65535 {
		return nil, errors.New("port must be 1 through 65535")
	}
	phase := time.Now()
	cacheDir := ""
	if config.Persistence.SavePath != "" {
		cacheDir = filepath.Join(filepath.Dir(config.Persistence.SavePath), "client-resource-cache")
	}
	resources, cacheState, err := loadClientResources(config.Resources, cacheDir)
	if cacheState == "" {
		cacheState = "miss"
	}
	logStartupPhase(config.Logger, "client_resources", phase, "cache", cacheState)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	saveDatabase, err := accountstore.OpenDatabase(config.Persistence.SavePath, config.Persistence.SeedPath, config.Logger)
	logStartupPhase(config.Logger, "open_database", phase)
	if err != nil {
		return nil, err
	}
	assembled := false
	defer func() {
		if !assembled {
			_ = saveDatabase.Close()
		}
	}()
	phase = time.Now()
	var pvpConfig game.PVPConfig
	if config.Masters.PVP != "" {
		pvpConfig, err = masterdata.LoadPVPRuntimeMaster(config.Masters.PVP)
		if err != nil {
			logStartupPhase(config.Logger, "policy_masters", phase)
			return nil, err
		}
	}
	playerProgression, err := masterdata.LoadPlayerProgressionRuntimeMaster(config.Masters.PlayerProgression)
	if err != nil {
		logStartupPhase(config.Logger, "policy_masters", phase)
		return nil, err
	}
	loginBonusPolicy, err := masterdata.LoadLoginBonusRuntimeMaster(config.Masters.LoginBonus)
	if err != nil {
		logStartupPhase(config.Logger, "policy_masters", phase)
		return nil, err
	}
	catalog, err := accountstore.LoadSaveState(config.Persistence.SeedPath)
	logStartupPhase(config.Logger, "policy_masters", phase)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	prepareRuntimeState, err := loadCNRuntimeStatePreparer(config.Masters, pvpConfig, playerProgression, loginBonusPolicy, resources.AvailableBundles, config.Logger)
	logStartupPhase(config.Logger, "runtime_masters", phase)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	catalog, err = prepareRuntimeState(catalog)
	logStartupPhase(config.Logger, "apply_seed_catalog", phase)
	if err != nil {
		return nil, err
	}
	saveDatabase.SetCatalog(catalog)
	phase = time.Now()
	primaryState, err := saveDatabase.LoadOrImport()
	logStartupPhase(config.Logger, "load_save", phase)
	if err != nil {
		return nil, err
	}
	accountStore, err := accountstore.NewAccounts(saveDatabase)
	if err != nil {
		return nil, err
	}
	if err := config.Multiplayer.AttachCompletionRepository(accountStore); err != nil {
		return nil, err
	}
	operationStore, err := adminapi.NewOperations(saveDatabase, primaryState.Gachas)
	if err != nil {
		return nil, err
	}
	if err := config.Multiplayer.AttachGameSpeed(operationStore.TeamBattleSpeed); err != nil {
		return nil, err
	}
	phase = time.Now()
	if err := operationStore.InitializeContent(catalog, config.Masters.Battle); err != nil {
		logStartupPhase(config.Logger, "operations_init", phase)
		return nil, err
	}
	if err := operationStore.InitializePlayerPolicy(catalog, config.Masters.Navi); err != nil {
		logStartupPhase(config.Logger, "operations_init", phase)
		return nil, err
	}
	logStartupPhase(config.Logger, "operations_init", phase)
	basePreparer := prepareRuntimeState
	prepareRuntimeState = func(state gamestate.State) (gamestate.State, error) {
		prepared, err := basePreparer(state)
		if err != nil {
			return prepared, err
		}
		return game.ApplyContentState(prepared, operationStore.ContentConfiguration()), nil
	}
	buildBusinessHandler := func(userID int, state gamestate.State) (http.Handler, error) {
		return newCNBusinessHandler(httpapi.Config{
			InitialState:        state,
			BaseURL:             "http://" + config.Network.AdvertiseHost + ":" + strconv.Itoa(config.Network.HTTPPort),
			Logger:              config.Logger,
			PersistState:        func(next gamestate.State) error { return accountStore.PersistState(userID, next) },
			PVP:                 pvpConfig,
			PVPAccounts:         accountStore,
			FriendPointAccounts: accountStore,
			BattleHistory:       accountStore,
			BattleSV:            config.Network.BattleSV,
			Multiplayer:         config.Multiplayer,
		}, prepareRuntimeState)
	}
	cacheLimit, err := accounthttp.ConfiguredAccountCacheLimit()
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	primaryBusinessHandler, err := buildBusinessHandler(accountstore.PrimaryUserID, primaryState)
	logStartupPhase(config.Logger, "primary_account", phase)
	if err != nil {
		return nil, err
	}
	businessHandler := accounthttp.New(accounthttp.Config{
		Primary:       primaryBusinessHandler,
		IdleLimit:     cacheLimit,
		Prepare:       operationStore.PrepareBusiness,
		PersistStates: accountStore.PersistStates,
		Build: func(userID int) (http.Handler, error) {
			state, err := accountStore.LoadState(userID)
			if err != nil {
				config.Logger.Error("load CN local account snapshot", "user_id", userID, "error", err)
				return nil, err
			}
			handler, err := buildBusinessHandler(userID, state)
			if err != nil {
				config.Logger.Error("build CN local account handler", "user_id", userID, "error", err)
				return nil, err
			}
			return handler, nil
		},
	})
	if err := config.Multiplayer.AttachStartAuthorizer(businessHandler.ChargeMultiplayerStart); err != nil {
		return nil, err
	}
	if err := config.Multiplayer.AttachContinueAuthorizer(businessHandler.ChargeMultiplayerContinue); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(config.Persistence.RequestLog)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("request log path is a directory")
	}
	r := &recorder{path: absolute}
	app := &application{
		config: config, resources: resources, accounts: accountStore,
		operations: operationStore, business: businessHandler, recorder: r, progression: playerProgression,
	}
	assembled = true
	return app, nil
}

func logStartupPhase(logger *slog.Logger, phase string, started time.Time, extra ...any) {
	if logger == nil {
		logger = slog.Default()
	}
	args := make([]any, 0, 4+len(extra))
	args = append(args, "phase", phase, "elapsed_ms", time.Since(started).Milliseconds())
	args = append(args, extra...)
	logger.Info("startup phase", args...)
}
