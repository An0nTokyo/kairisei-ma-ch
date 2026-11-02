package cnbootstrap

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"kairisei.local/server/internal/accountstore"
	"kairisei.local/server/internal/game"
	"kairisei.local/server/internal/gamestate"
	"kairisei.local/server/internal/httpapi"
	"kairisei.local/server/internal/masterdata"
)

// loadCNRuntimeStatePreparer reads immutable master data once per server.
func loadCNRuntimeStatePreparer(masters MastersConfig, pvpConfig game.PVPConfig, playerProgression gamestate.PlayerProgressionPolicy, loginBonusPolicy gamestate.LoginBonusPolicy, availableBundles map[string]struct{}, logger *slog.Logger) (func(gamestate.State) (gamestate.State, error), error) {
	phase := time.Now()
	cardMaster, err := masterdata.LoadCardRuntimeMaster(masters.Cards)
	logStartupPhase(logger, "master_cards", phase)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	exploreMaster, err := masterdata.LoadExploreRuntimeMaster(masters.Explore)
	logStartupPhase(logger, "master_explore", phase)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	storyMaster, err := masterdata.LoadStoryRuntimeMaster(masters.Story)
	logStartupPhase(logger, "master_story", phase)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	naviMaster, err := masterdata.LoadNaviRuntimeMaster(masters.Navi)
	logStartupPhase(logger, "master_navi", phase)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	itemMaster, err := masterdata.LoadItemRuntimeMaster(masters.Items)
	logStartupPhase(logger, "master_items", phase)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	avatarMaster, err := masterdata.LoadAvatarRuntimeMaster(masters.Avatar)
	logStartupPhase(logger, "master_avatar", phase)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	stampMaster, err := masterdata.LoadStampRuntimeMaster(masters.Stamps)
	logStartupPhase(logger, "master_stamps", phase)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	honorMaster, err := masterdata.LoadHonorRuntimeMaster(masters.Honors)
	logStartupPhase(logger, "master_honors", phase)
	if err != nil {
		return nil, err
	}
	var battleMaster masterdata.BattleRuntimeMaster
	var normalQuestMaster masterdata.NormalQuestRuntimeMaster
	if masters.Battle != "" {
		phase = time.Now()
		battleMaster, err = masterdata.LoadBattleRuntimeMaster(masters.Battle)
		logStartupPhase(logger, "master_battle", phase)
		if err != nil {
			return nil, err
		}
		normalQuestMasterPath := filepath.Join(
			filepath.Dir(masters.Battle),
			"cn602-normal-quest-runtime.json",
		)
		phase = time.Now()
		normalQuestMaster, err = masterdata.LoadNormalQuestRuntimeMaster(normalQuestMasterPath)
		logStartupPhase(logger, "master_normal_quest", phase)
		if err != nil {
			return nil, err
		}
	}
	return func(runtimeState gamestate.State) (gamestate.State, error) {
		var err error
		if runtimeState.User.Comment == "LOCAL OFFLINE PROFILE" {
			runtimeState.User.Comment = "请多关照！"
		}
		runtimeState.User.InviteID = accountstore.InviteID(runtimeState.User.UserID)
		runtimeState.User.CardContainerMax = max(runtimeState.User.CardContainerMax, gamestate.CardContainerCapacityDefault)
		runtimeState.User.SphereMax = max(runtimeState.User.SphereMax, gamestate.SphereCapacityDefault)
		runtimeState.User.BuddyMax = max(runtimeState.User.BuddyMax, gamestate.BuddyCapacityDefault)
		_, err = masterdata.ApplyPlayerProgressionRuntimeMaster(&runtimeState, playerProgression)
		if err != nil {
			return gamestate.State{}, err
		}
		_, err = masterdata.ApplyLoginBonusRuntimeMaster(&runtimeState, loginBonusPolicy)
		if err != nil {
			return gamestate.State{}, err
		}
		if pvpConfig.ConfigVersion > 0 {
			masterdata.NormalizePVPState(&runtimeState, pvpConfig)
		}
		normalizeCN602StampDeck(&runtimeState)
		_, err = masterdata.ApplyCardRuntimeMaster(&runtimeState, cardMaster)
		if err != nil {
			return gamestate.State{}, err
		}
		if err := masterdata.ApplyExploreRuntimeMaster(&runtimeState, exploreMaster); err != nil {
			return gamestate.State{}, err
		}
		masterdata.ApplyStoryRuntimeMaster(&runtimeState, storyMaster)
		if masters.Battle != "" {
			if err := masterdata.ApplyBattleRuntimeMaster(&runtimeState, battleMaster); err != nil {
				return gamestate.State{}, err
			}
			if err := masterdata.ApplyNormalQuestRuntimeMaster(&runtimeState, normalQuestMaster); err != nil {
				return gamestate.State{}, err
			}
		}
		if err := masterdata.ApplyNaviRuntimeMaster(&runtimeState, naviMaster); err != nil {
			return gamestate.State{}, err
		}
		_, err = masterdata.ApplyItemRuntimeMaster(&runtimeState, itemMaster)
		if err != nil {
			return gamestate.State{}, err
		}
		_, err = masterdata.ApplyAvatarRuntimeMaster(&runtimeState, avatarMaster)
		if err != nil {
			return gamestate.State{}, err
		}
		if err := masterdata.ApplyAvatarShopAssetClosure(&runtimeState, availableBundles); err != nil {
			return gamestate.State{}, err
		}
		_, err = masterdata.ApplyStampRuntimeMaster(&runtimeState, stampMaster)
		if err != nil {
			return gamestate.State{}, err
		}
		_, err = masterdata.ApplyHonorRuntimeMaster(&runtimeState, honorMaster)
		if err != nil {
			return gamestate.State{}, err
		}
		return runtimeState, nil

	}, nil
}

func newCNBusinessHandler(config httpapi.Config, prepareRuntimeState func(gamestate.State) (gamestate.State, error)) (http.Handler, error) {
	runtimeState := config.InitialState
	stateBefore, err := fingerprintCNAccountState(runtimeState)
	if err != nil {
		return nil, fmt.Errorf("encode CN runtime state before master application: %w", err)
	}
	phase := time.Now()
	runtimeState, err = prepareRuntimeState(runtimeState)
	logStartupPhase(config.Logger, "apply_account_masters", phase)
	if err != nil {
		return nil, err
	}
	phase = time.Now()
	contentGate, err := validateCNRunnableContent(runtimeState)
	logStartupPhase(config.Logger, "content_gate", phase)
	if err != nil {
		return nil, fmt.Errorf("validate CN runtime content gate: %w", err)
	}
	stateAfter, err := fingerprintCNAccountState(runtimeState)
	if err != nil {
		return nil, fmt.Errorf("encode CN runtime state after master application: %w", err)
	}
	if stateBefore != stateAfter {
		phase = time.Now()
		if err := config.PersistState(runtimeState); err != nil {
			logStartupPhase(config.Logger, "persist_migrated_state", phase)
			return nil, fmt.Errorf("persist validated CN runtime state migration: %w", err)
		}
		logStartupPhase(config.Logger, "persist_migrated_state", phase)
	}
	config.Logger.Info(
		"validated CN runtime content gate",
		"team_battle_groups", contentGate.TeamBattleGroups,
		"team_battle_boss_entries", contentGate.TeamBattleBossEntries,
		"user_buff_profiles", contentGate.UserBuffProfiles,
		"user_buff_references", contentGate.UserBuffReferences,
		"stage_quest_stages", contentGate.StageQuestStages,
		"explore_stages", contentGate.ExploreStages,
		"explore_floors", contentGate.ExploreFloors,
		"main_stories", contentGate.MainStories,
		"cn_main_stories", contentGate.CNMainStories,
		"sub_story_sections", contentGate.SubStorySections,
		"sub_stories", contentGate.SubStories,
		"story_events", contentGate.StoryEvents,
		"event_stories", contentGate.EventStories,
		"story_battle_ids", contentGate.StoryBattleIDs,
		"story_battle_references", contentGate.StoryBattleReferences,
		"popup_profiles", contentGate.PopupProfiles,
	)
	runtimeState.SchemaVersion = 1
	runtimeState.ReleaseID = "cn602-bootstrap"
	runtimeState.SourceBuild = "cn-official-6.0.2"
	runtimeState.CatalogVersion = cn602CatalogVersion
	runtimeState.State = "LOCAL"
	config.InitialState = runtimeState
	phase = time.Now()
	handler, err := httpapi.New(config)
	logStartupPhase(config.Logger, "build_http_handler", phase)
	return handler, err
}

func normalizeCN602StampDeck(state *gamestate.State) bool {
	const stampDeckSlots = 12
	if len(state.Stamps.DeckStampIDs) == stampDeckSlots {
		return false
	}
	deck := make([]int, stampDeckSlots)
	copy(deck, state.Stamps.DeckStampIDs)
	state.Stamps.DeckStampIDs = deck
	return true
}
