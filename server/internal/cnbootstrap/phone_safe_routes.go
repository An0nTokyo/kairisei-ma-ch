package cnbootstrap

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

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
	post("/UserProfileShow", app.phoneSafeUserProfileShow)
	post("/PvpShow", app.phoneSafePvpShow)
	// Task 4 appends PvpStart2 here.
	return paths
}

func writePhoneSafeRecorder(writer http.ResponseWriter, recorder *httptest.ResponseRecorder) {
	for k, vv := range recorder.Header() {
		for _, v := range vv {
			writer.Header().Add(k, v)
		}
	}
	writer.WriteHeader(recorder.Code)
	_, _ = writer.Write(recorder.Body.Bytes())
}

func phoneSafeForwardResCode(recorder *httptest.ResponseRecorder) (float64, bool) {
	if recorder.Code != http.StatusOK {
		return 0, false
	}
	lines := bytes.Split(bytes.TrimSpace(recorder.Body.Bytes()), []byte{'\n'})
	if len(lines) < 1 {
		return 0, false
	}
	var common map[string]any
	if err := json.Unmarshal(lines[0], &common); err != nil {
		return 0, false
	}
	code, ok := common["res_code"].(float64)
	return code, ok
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

func (app *application) phoneSafeUserProfileShow(writer http.ResponseWriter, request *http.Request) {
	payload, err := normalizeCNOptionalFields(request, "UserProfileShow", []string{"userid"}, []string{"userid"})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	recorder := httptest.NewRecorder()
	forwardCNBusiness(
		recorder,
		adaptCNBusinessRequest(request, http.MethodPost, "/UserProfileShow", payload),
		app.business,
		func(content []byte) ([]byte, error) {
			return adaptCNProtocolMethod(content, func(method map[string]json.RawMessage) (any, error) {
				return map[string]any{"profile": method["profile"]}, nil
			})
		},
	)
	if code, ok := phoneSafeForwardResCode(recorder); ok && code == 0 {
		writePhoneSafeRecorder(writer, recorder)
		return
	}
	writeCNProtocolResponseWithPopup(writer, cnBootstrapCommon(), map[string]any{
		"profile": map[string]any{
			"name": "未知玩家", "lv": 1, "hp": 1, "atkp": 1, "intp": 1, "mndp": 1,
			"deck": map[string]any{
				"arthur_type": 1, "is_burst": 0, "job_type": 1, "deck_rank": 1,
				"leader_cardid": 0, "leader_card_lv": 1, "leader_card_fame": 1,
			},
			"comment": "", "last_login_time": 0, "friend_state": 0, "pvp_point": 0,
			"deck_honorids": []int{},
		},
	})
}

func (app *application) phoneSafePvpShow(writer http.ResponseWriter, request *http.Request) {
	if err := requireCNSessionOnly(request, "PvpShow"); err != nil {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}
	recorder := httptest.NewRecorder()
	forwardCNBusiness(
		recorder,
		adaptCNBusinessRequest(request, http.MethodPost, "/PvpShow", nil),
		app.business,
		func(content []byte) ([]byte, error) { return content, nil },
	)
	if code, ok := phoneSafeForwardResCode(recorder); ok && code == 0 {
		writePhoneSafeRecorder(writer, recorder)
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
		"result": 0, "pvp_point": point, "challenge": challenge,
		"pvp_reset": map[string]any{"start_time": 0, "end_time": 0},
	})
}
