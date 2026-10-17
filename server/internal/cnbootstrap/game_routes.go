package cnbootstrap

import (
	"github.com/go-chi/chi/v5"
)

func (app *application) registerGameRoutes(router chi.Router) {
	app.registerSoloRoutes(router)
	router.Post("/TeamBattleMultiShow", cnBootstrapExactBusiness(
		app.business,
		"TeamBattleMultiShow",
		"/TeamBattleMultiShow",
		"active_arthur_type",
	))
	router.Post("/TeamBattleMultiRoomSearch", cnBootstrapExactBusiness(
		app.business,
		"TeamBattleMultiRoomSearch",
		"/TeamBattleMultiRoomSearch",
		"deck_arthur_type",
		"deck_arthur_type_idx",
		"pass",
		"is_rookie",
		"bossid",
		"boss_groupid",
		"rookie_type",
		"searchid",
		"quest_get_time",
		"is_auto",
		"empty_time",
	))
	router.Post("/TeamBattleMultiRoomCreate", cnBootstrapExactBusiness(
		app.business,
		"TeamBattleMultiRoomCreate",
		"/TeamBattleMultiRoomCreate",
		"bossid",
		"deck_arthur_type",
		"deck_arthur_type_idx",
		"room_type",
		"pass",
		"deck_rank",
		"hp",
		"fame",
		"comment",
		"flag",
		"allow_to_leave",
		"use_punished_free_piont",
	))
	router.Post("/TeamBattleAIRoomCreate", cnBootstrapExactBusiness(
		app.business,
		"TeamBattleAIRoomCreate",
		"/TeamBattleAIRoomCreate",
		"bossid",
		"ai_id",
		"deck_arthur_type",
		"deck_arthur_type_idx",
		"use_punished_free_piont",
	))
	router.Post("/TeamBattleMultiRoomReserve", cnBootstrapExactBusiness(
		app.business,
		"TeamBattleMultiRoomReserve",
		"/TeamBattleMultiRoomReserve",
		"roomid",
		"deck_arthur_type",
	))
	router.Post("/TeamBattleMultiRoomReserveCancel", cnBootstrapExactBusiness(
		app.business,
		"TeamBattleMultiRoomReserveCancel",
		"/TeamBattleMultiRoomReserveCancel",
		"roomid",
		"deck_arthur_type",
	))
	router.Post("/TeamBattleMultiRoomEnter", cnBootstrapExactBusiness(
		app.business,
		"TeamBattleMultiRoomEnter",
		"/TeamBattleMultiRoomEnter",
		"roomid",
		"deck_arthur_type",
		"deck_arthur_type_idx",
		"use_punished_free_piont",
	))
	router.Post("/TeamBattleResult", cnBootstrapTeamBattleResult(app.business))
	router.Post("/FollowoFollowShow", cnBootstrapFollowShow(app.business))
	router.Post("/FollowFollowerShow", cnBootstrapFollowerShow(app.business))
	router.Post("/FriendSearch", cnBootstrapFriendSearch(app.business))
	router.Post("/FollowAdd", cnBootstrapFollowAdd(app.business))
	router.Post("/FollowUnFollow", cnBootstrapFollowUnfollow(app.business))
	router.Post("/UserProfileShow", cnBootstrapUserProfileShow(app.business))
	router.Post("/PvpShow", cnBootstrapSessionOnlyBusiness(app.business, "PvpShow", "POST", "/PvpShow"))
	router.Post("/PvpStart2", cnBootstrapExactBusiness(app.business, "PvpStart2", "/PvpStart2", "type", "select_arthur_type", "pvp_my_deck"))
	router.Post("/PvpEnd", cnBootstrapExactBusiness(app.business, "PvpEnd", "/PvpEnd", "btluid", "is_win", "is_retire", "input_cmd"))
}
