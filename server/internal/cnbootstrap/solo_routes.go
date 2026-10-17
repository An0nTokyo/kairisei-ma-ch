package cnbootstrap

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *application) registerSoloRoutes(router chi.Router) []string {
	var paths []string
	post := func(path string, handler http.HandlerFunc) {
		paths = append(paths, path)
		router.Post(path, handler)
	}
	post("/Ping", cnBootstrapPing)
	post("/Connect", cnBootstrapConnect(app.business))
	post("/HomeShow", cnBootstrapSessionOnlyBusiness(app.business, "HomeShow", http.MethodPost, "/HomeShow"))
	post("/PopupExec", cnBootstrapExactBusiness(app.business, "PopupExec", "/PopupExec", "popupid", "is_system"))
	post("/TutorialProgress", cnBootstrapTutorialProgress)
	post("/TutorialFlag", cnBootstrapTutorialFlag(app.business))
	post("/ClickLog", cnBootstrapClickLog)
	post("/ClickNewVersionInfo", cnBootstrapClickNewVersionInfo)
	post("/UserCreate", cnBootstrapExactBusiness(app.business, "UserCreate", "/UserCreate", "name", "arthur_type"))
	post("/UserSetName", cnBootstrapExactBusiness(app.business, "UserSetName", "/UserSetName", "name"))
	post("/UserSetComment", cnBootstrapExactBusiness(app.business, "UserSetComment", "/UserSetComment", "comment"))
	post("/GetMobileServiceToken", cnBootstrapExactBusiness(app.business, "GetMobileServiceToken", "/__domain/MobileServiceInfo", "sprite"))
	post("/ItemShow", cnBootstrapItemShow(app.business))
	post("/ItemUse", cnBootstrapExactBusiness(app.business, "ItemUse", "/ItemUse", "itemid"))
	post("/ItemExchange", cnPayloadAdapter(app.business, "ItemExchange", "/ItemExchange", []string{"itemid", "change_sets"}, adaptCNNamedRewardMethod))
	post("/ItemLackTips", cnBootstrapExactBusiness(app.business, "ItemLackTips", "/ItemLackTips", "idx"))
	post("/ItemShopShow", cnBootstrapSessionOnlyBusiness(app.business, "ItemShopShow", http.MethodPost, "/ItemShopShow"))
	post("/ItemShopBuy", cnBootstrapExactBusiness(app.business, "ItemShopBuy", "/ItemShopBuy", "item_shop_lineupid", "buy_num", "popupid"))
	post("/EventShopShow", cnBootstrapExactBusiness(app.business, "EventShopShow", "/EventShopShow", "eventid"))
	post("/EventShopBuy", cnBootstrapExactBusiness(app.business, "EventShopBuy", "/EventShopBuy", "event_shop_lineupid"))
	post("/GachaShow", cnBootstrapGachaShow(app.business, app.operations))
	post("/GachaPlay2", cnBootstrapGachaPlay(app.business, app.operations))
	post("/GachaItemPlay", cnPayloadAdapter(app.business, "GachaItemPlay", "/GachaItemPlay", []string{"itemid", "play_count", "gacha_hash"}, adaptCNGachaItemPlayMethod))
	post("/GachaLineupShow2", cnBootstrapGachaLineupShow(app.business, app.operations))
	gachaPublicationBusiness := cnGachaPublicationBusiness(app.business, app.operations)
	post("/GachaSelectLineupShow", cnBootstrapExactBusiness(gachaPublicationBusiness, "GachaSelectLineupShow", "/GachaSelectLineupShow", "gachaid"))
	post("/GachaSelectedListShow", cnBootstrapExactBusiness(gachaPublicationBusiness, "GachaSelectedListShow", "/GachaSelectedListShow", "gachaid"))
	post("/GachaOddsShow", cnBootstrapGachaOddsShow(app.business, app.operations))
	post("/GetRecommendCardInfo", cnBootstrapSessionOnlyBusiness(app.business, "GetRecommendCardInfo", http.MethodPost, "/GetRecommendCardInfo"))
	post("/GetURCardNoGetFromCurrentGaCha", cnBootstrapSessionOnlyBusiness(gachaPublicationBusiness, "GetURCardNoGetFromCurrentGaCha", http.MethodPost, "/GetURCardNoGetFromCurrentGaCha"))
	post("/CoinUse", cnBootstrapExactBusiness(app.business, "CoinUse", "/CoinUse", "type", "param"))
	post("/TradeShopShow2", cnBootstrapSessionOnlyBusiness(app.business, "TradeShopShow2", http.MethodPost, "/TradeShopShow2"))
	post("/TradeShopLineupShow", cnBootstrapExactBusiness(app.business, "TradeShopLineupShow", "/TradeShopLineupShow", "trade_shopid"))
	post("/TradeShopBuy2", cnBootstrapTradeShopBuy2(app.business))
	post("/PurchaseBonusActivityShow", cnBootstrapSessionOnlyBusiness(app.business, "PurchaseBonusActivityShow", http.MethodPost, "/PurchaseBonusActivityShow"))
	post("/QueryOrder", cnBootstrapExactBusiness(app.business, "QueryOrder", "/QueryOrder", "order"))
	post("/QueryCardCoin", cnBootstrapSessionOnlyBusiness(app.business, "QueryCardCoin", http.MethodPost, "/QueryCardCoin"))
	post("/GetCardCoin", cnBootstrapExactBusiness(app.business, "GetCardCoin", "/GetCardCoin", "cardtype"))
	post("/HonorShow", cnBootstrapHonorShow(app.business))
	post("/HonorDeckShow", cnBootstrapSessionOnlyBusiness(app.business, "HonorDeckShow", http.MethodPost, "/HonorDeckShow"))
	post("/HonorDeckSet", cnBootstrapExactBusiness(app.business, "HonorDeckSet", "/HonorDeckSet", "deck_honorids"))
	post("/CardCollectionShow", cnBootstrapCardCollectionShow(app.business))
	post("/CardShow2", cnBootstrapCardShow2(app.business))
	post("/CardContainerShow", cnBootstrapCardContainerShow(app.business))
	post("/CardMove", cnBootstrapCardMove(app.business))
	post("/CardContainerLock", cnBootstrapExactBusiness(app.business, "CardContainerLock", "/CardContainerLock", "uniqid"))
	post("/CardContainerUnlock", cnBootstrapExactBusiness(app.business, "CardContainerUnlock", "/CardContainerUnlock", "uniqid"))
	post("/CardContainerSell", cnBootstrapExactBusiness(app.business, "CardContainerSell", "/CardContainerSell", "uniqids"))
	post("/CardLock", cnBootstrapExactBusiness(app.business, "CardLock", "/CardLock", "uniqid"))
	post("/CardUnlock", cnBootstrapExactBusiness(app.business, "CardUnlock", "/CardUnlock", "uniqid"))
	post("/CardLoveUp", cnBootstrapCardLoveUp(app.business))
	post("/CardDecompose", cnBootstrapCardDecompose(app.business))
	post("/CardFameTrainInfo", cnBootstrapSessionOnlyBusiness(app.business, "CardFameTrainInfo", http.MethodPost, "/CardFameTrainInfo"))
	post("/CardFameStartTrain", cnBootstrapExactBusiness(app.business, "CardFameStartTrain", "/CardFameStartTrain", "base_uniqid", "base_fame"))
	post("/CardFameCancelTrain", cnBootstrapExactBusiness(app.business, "CardFameCancelTrain", "/CardFameCancelTrain", "base_uniqid"))
	post("/CardFameTrainFinish", cnBootstrapExactBusiness(app.business, "CardFameTrainFinish", "/CardFameTrainFinish", "base_uniqid"))
	post("/HowToGetCardShow", cnBootstrapHowToGetCardShow(app.business, app.operations))
	post("/CardCategoryGet", cnBootstrapCardCategoryGet(app.business))
	post("/CardDeckSet", cnBootstrapCardDeckSet(app.business))
	post("/SupportCardSlotUnlock", cnBootstrapExactBusiness(app.business, "SupportCardSlotUnlock", "/SupportCardSlotUnlock", "arthur_type"))
	post("/CardFusion2", cnBootstrapCardFusion2(app.business))
	post("/CardEvolution", cnBootstrapCardEvolution(app.business))
	post("/CardSell", cnBootstrapCardSell(app.business))
	post("/ExploreStart", cnBootstrapExploreStart(app.business))
	post("/ExploreEnd", cnBootstrapExploreEnd(app.business))
	post("/MissionShow", cnBootstrapMissionShow(app.business))
	post("/MissionReward", cnBootstrapMissionReward(app.business))
	post("/MissionURLOpen", cnBootstrapExactBusiness(app.business, "MissionURLOpen", "/MissionURLOpen", "open_url"))
	post("/PresentBoxShow", cnBootstrapPresentBoxShow(app.business))
	post("/PresentBoxRecv", cnBootstrapPresentBoxRecv(app.business))
	post("/PresentBoxMultiRecv2", cnBootstrapPresentBoxMultiRecv(app.business))
	post("/PresentBoxDelete", cnBootstrapExactBusiness(app.business, "PresentBoxDelete", "/PresentBoxDelete", "presentid"))
	post("/UpdGameOption", cnBootstrapExactBusiness(app.business, "UpdGameOption", "/UpdGameOption", "game_option"))
	post("/UpdPushOption", cnBootstrapExactBusiness(app.business, "UpdPushOption", "/UpdPushOption", "option"))
	post("/GetNaviShow", cnBootstrapSessionOnlyBusiness(app.business, "GetNaviShow", http.MethodPost, "/GetNaviShow"))
	post("/NaviSelect", cnBootstrapExactBusiness(app.business, "NaviSelect", "/NaviSelect", "navi_type"))
	post("/BuyNavi", cnBootstrapExactBusiness(app.business, "BuyNavi", "/BuyNavi", "navi_id"))
	post("/StampShow", cnBootstrapSessionOnlyBusiness(app.business, "StampShow", http.MethodPost, "/StampShow"))
	post("/StampDeckSet", cnBootstrapExactBusiness(app.business, "StampDeckSet", "/StampDeckSet", "deck"))
	post("/CostumeShow", cnBootstrapSessionOnlyBusiness(app.business, "CostumeShow", http.MethodPost, "/CostumeShow"))
	post("/CostumeSet", cnBootstrapExactBusiness(app.business, "CostumeSet", "/CostumeSet", "arthur_type", "costumeid"))
	post("/AvatarPartsShow", cnBootstrapSessionOnlyBusiness(app.business, "AvatarPartsShow", http.MethodPost, "/AvatarPartsShow"))
	post("/AvatarPartsDeckSet", cnBootstrapExactBusiness(app.business, "AvatarPartsDeckSet", "/AvatarPartsDeckSet", "avatar_parts_decks"))
	post("/AvatarShopShow", cnBootstrapSessionOnlyBusiness(app.business, "AvatarShopShow", http.MethodPost, "/AvatarShopShow"))
	post("/AvatarShopBuy", cnBootstrapExactBusiness(app.business, "AvatarShopBuy", "/AvatarShopBuy", "avatar_shop_lineupid", "sales_index"))
	post("/DeckLimitShow", cnBootstrapSessionOnlyBusiness(app.business, "DeckLimitShow", http.MethodPost, "/DeckLimitShow"))
	post("/TeamBattleSoloShow", cnBootstrapTeamBattleSoloShow(app.business, app.operations))
	post("/TowerQuestShow", cnBootstrapExactBusiness(app.business, "TowerQuestShow", "/TowerQuestShow", "towerid"))
	post("/TowerRankingShow", cnBootstrapExactBusiness(app.business, "TowerRankingShow", "/TowerRankingShow", "towerid"))
	post("/TeamBattleSoloPartnerShow", cnBootstrapExactBusiness(app.business, "TeamBattleSoloPartnerShow", "/TeamBattleSoloPartnerShow", "bossid"))
	post("/TeamBattleSoloPartnerRentalDeck", cnBootstrapExactBusiness(app.business, "TeamBattleSoloPartnerRentalDeck", "/TeamBattleSoloPartnerRentalDeck", "userid"))
	post("/TeamBattleRecommendDeckShow", cnBootstrapSessionOnlyBusiness(app.business, "TeamBattleRecommendDeckShow", http.MethodPost, "/TeamBattleRecommendDeckShow"))
	post("/TeamBattlePastBossShow", cnBootstrapPastBossShow(app.business, app.operations))
	post("/TeamBattleClearDeckShow", cnBootstrapExactBusiness(cnCardAcquisitionPublication(app.business, app.operations), "TeamBattleClearDeckShow", "/TeamBattleClearDeckShow", "bossid"))
	post("/TeamBattleScoreRewardLineup", cnBootstrapExactBusiness(app.business, "TeamBattleScoreRewardLineup", "/TeamBattleScoreRewardLineup", "bossid"))
	post("/DailyClearRankShow", cnBootstrapExactBusiness(cnCardAcquisitionPublication(app.business, app.operations), "DailyClearRankShow", "/DailyClearRankShow", "bossid", "is_multi"))
	post("/ChallengeShow", cnBootstrapExactBusiness(app.business, "ChallengeShow", "/ChallengeShow", "bossid", "target_time"))
	post("/TeamBattleScheduleShow", cnBootstrapExactBusiness(app.business, "TeamBattleScheduleShow", "/TeamBattleScheduleShow", "is_solo", "active_arthur_type"))
	post("/TeamBattleScheduleUpdate", cnBootstrapExactBusiness(app.business, "TeamBattleScheduleUpdate", "/TeamBattleScheduleUpdate", "is_solo", "boss_groupid"))
	post("/UserBuffExec", cnBootstrapExactBusiness(app.business, "UserBuffExec", "/UserBuffExec", "user_buff_id"))
	post("/TeamBattleSoloStart", cnBootstrapExactBusiness(
		app.business,
		"TeamBattleSoloStart",
		"/TeamBattleSoloStart",
		"bossid",
		"deck_arthur_type",
		"deck_arthur_type_idx",
		"partner_deck_selects",
		"starttime",
		"flag",
	))
	post("/TeamBattleSoloContinue", cnBootstrapExactBusiness(
		app.business,
		"TeamBattleSoloContinue",
		"/TeamBattleSoloContinue",
		"pay_type",
		"progress",
		"input_cmd",
		"enemy_dead_bit",
	))
	post("/TeamBattleSoloEnd", cnBootstrapTeamBattleSoloEnd(app.business))
	post("/StageQuestShow", cnBootstrapExactBusiness(
		app.business,
		"StageQuestShow",
		"/StageQuestShow",
		"areaid",
		"is_solo",
	))
	post("/SphrShow", cnBootstrapSphereShow(app.business))
	post("/SphrFusion2", cnBootstrapSphereFusion2(app.business))
	post("/SphrEvolution", cnBootstrapSphereEvolution(app.business))
	post("/SphrSell", cnBootstrapSphereSell(app.business))
	post("/SphrLock", cnBootstrapSphereLock(app.business, false))
	post("/SphrUnlock", cnBootstrapSphereLock(app.business, true))
	post("/BuddyShow", cnBootstrapBuddyShow(app.business))
	post("/BuddyFusion", cnBootstrapBuddyFusion(app.business))
	post("/BuddyEvolution", cnBootstrapBuddyEvolution(app.business))
	post("/BuddySell", cnBootstrapBuddySell(app.business))
	post("/BuddyLock", cnBootstrapBuddyLock(app.business, false))
	post("/BuddyUnlock", cnBootstrapBuddyLock(app.business, true))
	post("/StoryMainShow", cnBootstrapStoryMainShow(app.business))
	post("/StoryMainStart", cnBootstrapStoryMainStart(app.business))
	post("/StoryMainEnd", cnBootstrapStoryMainEnd(app.business))
	post("/StorySubShow", cnBootstrapStorySubShow(app.business))
	post("/StorySubStart", cnBootstrapStorySubStart(app.business))
	post("/StorySubEnd", cnBootstrapStorySubEnd(app.business))
	post("/StoryEventShow", cnBootstrapStoryEventShow(app.business))
	post("/StoryEventUnlock", cnBootstrapStoryEventUnlock(app.business))
	post("/StoryStart", cnBootstrapStoryStart(app.business))
	post("/StoryTeamBattleStart", cnBootstrapExactBusiness(app.business,
		"StoryTeamBattleStart", "/StoryTeamBattleStart", "story_teambattleid"))
	post("/StoryTeamBattleEnd", cnBootstrapStoryTeamBattleEnd(app.business))
	post("/EventShow", cnBootstrapSessionOnlyBusiness(app.business, "EventShow", http.MethodPost, "/EventShow"))
	return paths
}
