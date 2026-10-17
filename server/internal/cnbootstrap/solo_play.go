package cnbootstrap

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"kairisei.local/server/internal/accounthttp"
	"kairisei.local/server/internal/accountstore"
	adminapi "kairisei.local/server/internal/admin"
	"kairisei.local/server/internal/game"
	"kairisei.local/server/internal/gamestate"
	"kairisei.local/server/internal/httpapi"
	"kairisei.local/server/internal/masterdata"
	"kairisei.local/server/internal/multiplayer"
)

type SoloPlayConfig struct {
	DeviceID     string
	BaseURL      string
	Logger       *slog.Logger
	Masters      MastersConfig
	BattleSV     multiplayer.Endpoint
	DefenseDecks []byte
}

func NewSoloPlay(accounts *accountstore.Accounts, config SoloPlayConfig) (http.Handler, []string, error) {
	if accounts == nil {
		return nil, nil, fmt.Errorf("CN solo play accounts are required")
	}
	if config.Logger == nil {
		config.Logger = slog.New(slog.DiscardHandler)
	}
	if config.BaseURL == "" {
		return nil, nil, fmt.Errorf("CN solo play base URL is required")
	}
	if config.DeviceID == "" {
		return nil, nil, fmt.Errorf("CN solo play device is required")
	}
	bundles, err := avatarBundles(config.Masters.Avatar)
	if err != nil {
		return nil, nil, err
	}
	var pvpConfig game.PVPConfig
	if config.Masters.PVP != "" {
		pvpConfig, err = masterdata.LoadPVPRuntimeMaster(config.Masters.PVP)
		if err != nil {
			return nil, nil, err
		}
	}
	playerProgression, err := masterdata.LoadPlayerProgressionRuntimeMaster(config.Masters.PlayerProgression)
	if err != nil {
		return nil, nil, err
	}
	loginBonus, err := masterdata.LoadLoginBonusRuntimeMaster(config.Masters.LoginBonus)
	if err != nil {
		return nil, nil, err
	}
	prepare, err := loadCNRuntimeStatePreparer(config.Masters, pvpConfig, playerProgression, loginBonus, bundles, config.Logger)
	if err != nil {
		return nil, nil, err
	}
	catalog, err := accountstore.LoadSaveState(accounts.Database().SeedPath())
	if err != nil {
		return nil, nil, err
	}
	catalog, err = prepare(catalog)
	if err != nil {
		return nil, nil, err
	}
	accounts.Database().SetCatalog(catalog)
	hub := multiplayer.NewHub()
	if err := hub.AttachCompletionRepository(accounts); err != nil {
		return nil, nil, err
	}
	operations, err := adminapi.NewOperations(accounts.Database(), catalog.Gachas)
	if err != nil {
		return nil, nil, err
	}
	if err := hub.AttachGameSpeed(operations.TeamBattleSpeed); err != nil {
		return nil, nil, err
	}
	if err := operations.InitializeContent(catalog, config.Masters.Battle); err != nil {
		return nil, nil, err
	}
	if err := operations.InitializePlayerPolicy(catalog, config.Masters.Navi); err != nil {
		return nil, nil, err
	}
	basePrepare := prepare
	prepare = func(state gamestate.State) (gamestate.State, error) {
		prepared, err := basePrepare(state)
		if err != nil {
			return prepared, err
		}
		return game.ApplyContentState(prepared, operations.ContentConfiguration()), nil
	}
	cacheLimit, err := accounthttp.ConfiguredAccountCacheLimit()
	if err != nil {
		return nil, nil, err
	}
	opponents, err := frozenPVPOpponents(config.DefenseDecks)
	if err != nil {
		return nil, nil, err
	}
	business := accounthttp.New(accounthttp.Config{
		IdleLimit:     cacheLimit,
		Prepare:       operations.PrepareBusiness,
		PersistStates: accounts.PersistStates,
		Build: func(userID int) (http.Handler, error) {
			state, err := accounts.LoadState(userID)
			if err != nil {
				return nil, err
			}
			return newCNBusinessHandler(httpapi.Config{
				InitialState:        state,
				BaseURL:             config.BaseURL,
				Logger:              config.Logger,
				PersistState:        func(next gamestate.State) error { return accounts.PersistState(userID, next) },
				PVP:                 pvpConfig,
				PVPAccounts:         opponents.repository(accounts),
				FriendPointAccounts: accounts,
				BattleHistory:       accounts,
				BattleSV:            config.BattleSV,
				Multiplayer:         hub,
			}, prepare)
		},
	})
	app := &application{accounts: accounts, operations: operations, business: business}
	router := chi.NewRouter()
	router.Use(authenticateCNSessions(accounts))
	router.Use(requirePhoneHolder(accounts, config.DeviceID))
	paths := app.registerSoloRoutes(router)
	if opponents.ready {
		paths = append(paths, app.registerDefenseRoutes(router)...)
	}
	return router, paths, nil
}

type frozenOpponentSource struct {
	ready  bool
	states []gamestate.State
}

func frozenPVPOpponents(raw []byte) (frozenOpponentSource, error) {
	if len(raw) == 0 {
		return frozenOpponentSource{}, nil
	}
	var states []gamestate.State
	if err := json.Unmarshal(raw, &states); err != nil {
		return frozenOpponentSource{}, fmt.Errorf("CN solo defense decks: %w", err)
	}
	return frozenOpponentSource{ready: true, states: states}, nil
}

func (source frozenOpponentSource) repository(fallback interface {
	ListPVPOpponents(userID int) ([]gamestate.State, error)
}) interface {
	ListPVPOpponents(userID int) ([]gamestate.State, error)
} {
	if !source.ready {
		return fallback
	}
	return source
}

func (source frozenOpponentSource) ListPVPOpponents(userID int) ([]gamestate.State, error) {
	result := make([]gamestate.State, 0, len(source.states))
	for _, state := range source.states {
		if state.User.UserID != userID {
			result = append(result, state)
		}
	}
	return result, nil
}

func avatarBundles(path string) (map[string]struct{}, error) {
	master, err := masterdata.LoadAvatarRuntimeMaster(path)
	if err != nil {
		return nil, err
	}
	bundles := map[string]struct{}{
		"avatar_parts/icon/avatar_00000_icon.dat": {},
	}
	for _, part := range master.PartDefinitions {
		bundles[fmt.Sprintf("avatar_parts/icon/avatar_%05d_icon.dat", part.IconPictID)] = struct{}{}
		bundles[fmt.Sprintf("avatar_parts/parts/avatar_%05d_parts.dat", part.PartID)] = struct{}{}
	}
	return bundles, nil
}

func requirePhoneHolder(accounts *accountstore.Accounts, deviceID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			userID, ok := request.Context().Value(cnAuthenticatedUserKey{}).(int)
			if ok && accounts.RefuseIfNotHolder(userID, deviceID) != nil {
				writer.Header().Set("Content-Type", "application/json; charset=utf-8")
				_ = json.NewEncoder(writer).Encode(map[string]any{
					"res_code": -1,
					"res_str":  "该账号不在这台手机上",
				})
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}
