package cnbootstrap

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *application) registerPhoneSafeRoutes(router chi.Router) []string {
	var paths []string
	post := func(path string, handler http.HandlerFunc) {
		paths = append(paths, path)
		router.Post(path, handler)
	}
	post("/FollowoFollowShow", phoneSafeFollowShow)
	post("/FollowFollowerShow", phoneSafeFollowerShow)
	post("/FriendSearch", phoneSafeFriendSearch)
	post("/FollowAdd", phoneSafeFollowAdd)
	post("/FollowUnFollow", phoneSafeFollowUnfollow)
	post("/PvpEnd", app.phoneSafePvpEnd)
	// Tasks 3–4 append more posts here (or register all nine in one file as you implement).
	return paths
}

func phoneSafeFollowShow(writer http.ResponseWriter, request *http.Request) {
	if err := requireCNSessionOnly(request, "FollowoFollowShow"); err != nil {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}
	writeCNProtocolResponseWithPopup(writer, cnBootstrapCommon(), map[string]any{
		"follow_max": 80,
		"friends":    []any{},
	})
}

func phoneSafeFollowerShow(writer http.ResponseWriter, request *http.Request) {
	if _, err := normalizeCNOptionalFields(request, "FollowFollowerShow", []string{"page"}, []string{"page"}); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writeCNProtocolResponseWithPopup(writer, cnBootstrapCommon(), map[string]any{
		"friends":      []any{},
		"follower_num": 0,
		"page_max":     1,
	})
}

func phoneSafeFriendSearch(writer http.ResponseWriter, request *http.Request) {
	if _, err := normalizeCNOptionalFields(request, "FriendSearch", []string{"inviteid"}, []string{"inviteid"}); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writeCNProtocolResponseWithPopup(writer, cnBootstrapCommon(), map[string]any{
		"user": []any{},
	})
}

func phoneSafeFollowAdd(writer http.ResponseWriter, request *http.Request) {
	payload, err := normalizeCNOptionalFields(request, "FollowAdd", []string{"userids"}, []string{"userids"})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	var requestData struct {
		UserIDs []int `json:"userids"`
	}
	if err := json.Unmarshal(payload, &requestData); err != nil {
		http.Error(writer, "invalid CN FollowAdd payload", http.StatusBadRequest)
		return
	}
	_ = requestData // intentionally ignore targets
	writeCNProtocolResponseWithPopup(writer, cnBootstrapCommon(), map[string]any{
		"request_userids": []int{},
		"is_friend_full":  0,
	})
}

func phoneSafeFollowUnfollow(writer http.ResponseWriter, request *http.Request) {
	if _, err := normalizeCNOptionalFields(request, "FollowUnFollow", []string{"userid"}, []string{"userid"}); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writeCNProtocolResponseWithPopup(writer, cnBootstrapCommon(), map[string]any{})
}

func (app *application) phoneSafePvpEnd(writer http.ResponseWriter, request *http.Request) {
	payload, err := normalizeCNOptionalFields(request, "PvpEnd",
		[]string{"btluid", "is_win", "is_retire", "input_cmd"},
		[]string{"btluid", "is_win", "is_retire", "input_cmd"})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		BattleID int      `json:"btluid"`
		IsWin    int8     `json:"is_win"`
		IsRetire int8     `json:"is_retire"`
		InputCmd []string `json:"input_cmd"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		http.Error(writer, "invalid CN PvpEnd payload", http.StatusBadRequest)
		return
	}
	if (body.IsWin != 0 && body.IsWin != 1) || (body.IsRetire != 0 && body.IsRetire != 1) {
		http.Error(writer, "invalid PVP settlement flags", http.StatusBadRequest)
		return
	}
	point, challenge := 0, 0
	if userID, ok := request.Context().Value(cnAuthenticatedUserKey{}).(int); ok {
		if state, err := app.accounts.LoadState(userID); err == nil {
			point = state.User.PVPPoint
			challenge = state.PVP.Challenge
		}
	}
	writeCNProtocolResponseWithPopup(writer, cnBootstrapCommon(), map[string]any{
		"result":       0,
		"bf_pvp_point": point,
		"af_pvp_point": point,
		"challenge":    challenge,
	})
}
