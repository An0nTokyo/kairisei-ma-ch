package solo_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kairisei.local/mserver/internal/degrade"
)

func TestPhoneSafeFriendListsAreEmptyNotDegraded(t *testing.T) {
	store, userID, handler := openDefensePhone(t, false)
	session := phoneSession(t, store, userID)
	cases := []struct {
		path string
		body string
		want map[string]any
	}{
		{"/FollowoFollowShow", "", map[string]any{"follow_max": float64(80), "friends": []any{}}},
		{"/FollowFollowerShow", `{"page":0}`, map[string]any{"friends": []any{}, "follower_num": float64(0), "page_max": float64(1)}},
		{"/FriendSearch", `{"inviteid":"x"}`, map[string]any{"user": []any{}}},
	}
	for _, tc := range cases {
		response := postPhone(handler, tc.path, session+tc.body)
		if strings.Contains(response.Body.String(), degrade.Message) {
			t.Fatalf("%s degraded: %s", tc.path, response.Body.String())
		}
		common, method := decodeCNPhoneSafe(t, response)
		if common["res_code"] != float64(0) {
			t.Fatalf("%s res_code=%v body=%s", tc.path, common["res_code"], response.Body.String())
		}
		for key, want := range tc.want {
			got, _ := json.Marshal(method[key])
			exp, _ := json.Marshal(want)
			if string(got) != string(exp) {
				t.Fatalf("%s field %s = %s want %s", tc.path, key, got, exp)
			}
		}
	}
}

func TestPhoneSafeFollowAndPvpEndDoNotMutate(t *testing.T) {
	store, userID, handler := openDefensePhone(t, false)
	session := phoneSession(t, store, userID)
	before, err := store.SnapshotRevision(userID)
	if err != nil {
		t.Fatal(err)
	}
	add := postPhone(handler, "/FollowAdd", session+`{"userids":[1000002,0,0,0,0,0,0,0,0,0]}`)
	if strings.Contains(add.Body.String(), degrade.Message) {
		t.Fatalf("FollowAdd degraded: %s", add.Body.String())
	}
	common, method := decodeCNPhoneSafe(t, add)
	if common["res_code"] != float64(0) {
		t.Fatalf("FollowAdd res_code=%v", common["res_code"])
	}
	if string(mustJSON(t, method["request_userids"])) != "[]" || method["is_friend_full"] != float64(0) {
		t.Fatalf("FollowAdd method=%v", method)
	}
	un := postPhone(handler, "/FollowUnFollow", session+`{"userid":1000002}`)
	common, _ = decodeCNPhoneSafe(t, un)
	if common["res_code"] != float64(0) || strings.Contains(un.Body.String(), degrade.Message) {
		t.Fatalf("FollowUnFollow=%s", un.Body.String())
	}
	end := postPhone(handler, "/PvpEnd", session+`{"btluid":1,"is_win":0,"is_retire":1,"input_cmd":[""]}`)
	common, method = decodeCNPhoneSafe(t, end)
	if common["res_code"] != float64(0) || strings.Contains(end.Body.String(), degrade.Message) {
		t.Fatalf("PvpEnd=%s", end.Body.String())
	}
	bf, _ := method["bf_pvp_point"].(float64)
	af, _ := method["af_pvp_point"].(float64)
	if bf != af {
		t.Fatalf("PvpEnd changed points bf=%v af=%v", bf, af)
	}
	after, err := store.SnapshotRevision(userID)
	if err != nil || after != before {
		t.Fatalf("revision %d -> %d", before, after)
	}
}

func TestPhoneSafeProfileAndPvpShowNotDegraded(t *testing.T) {
	store, userID, handler := openDefensePhone(t, false)
	session := phoneSession(t, store, userID)
	self := postPhone(handler, "/UserProfileShow", session+fmt.Sprintf(`{"userid":%d}`, userID))
	if strings.Contains(self.Body.String(), degrade.Message) {
		t.Fatalf("self profile degraded: %s", self.Body.String())
	}
	common, method := decodeCNPhoneSafe(t, self)
	if common["res_code"] != float64(0) {
		t.Fatalf("self profile res_code=%v body=%s", common["res_code"], self.Body.String())
	}
	if _, ok := method["profile"]; !ok {
		t.Fatalf("self profile missing: %v", method)
	}
	other := postPhone(handler, "/UserProfileShow", session+`{"userid":1999999}`)
	if strings.Contains(other.Body.String(), degrade.Message) {
		t.Fatalf("other profile degraded: %s", other.Body.String())
	}
	common, method = decodeCNPhoneSafe(t, other)
	if common["res_code"] != float64(0) {
		t.Fatalf("other profile res_code=%v body=%s", common["res_code"], other.Body.String())
	}
	if _, ok := method["profile"]; !ok {
		t.Fatalf("other profile missing: %v", method)
	}
	show := postPhone(handler, "/PvpShow", session)
	if strings.Contains(show.Body.String(), degrade.Message) {
		t.Fatalf("PvpShow degraded: %s", show.Body.String())
	}
	common, method = decodeCNPhoneSafe(t, show)
	if common["res_code"] != float64(0) {
		t.Fatalf("PvpShow res_code=%v body=%s", common["res_code"], show.Body.String())
	}
	for _, key := range []string{"result", "pvp_point", "challenge", "pvp_reset"} {
		if _, ok := method[key]; !ok {
			t.Fatalf("PvpShow missing %s: %v", key, method)
		}
	}
}

func TestPhoneSafePvpStartWithoutDefenseReturnsMinus1200(t *testing.T) {
	store, userID, handler := openDefensePhone(t, false)
	session := phoneSession(t, store, userID)
	body := `{"type":0,"select_arthur_type":1,"pvp_my_deck":[]}`
	response := postPhone(handler, "/PvpStart2", session+body)
	if strings.Contains(response.Body.String(), degrade.Message) {
		t.Fatalf("degraded: %s", response.Body.String())
	}
	common, _ := decodeCNPhoneSafe(t, response)
	if common["res_code"] != float64(-1200) {
		t.Fatalf("res_code=%v want -1200 body=%s", common["res_code"], response.Body.String())
	}
}

func TestUnknownRouteStillDegraded(t *testing.T) {
	store, userID, handler := openDefensePhone(t, false)
	session := phoneSession(t, store, userID)
	response := postPhone(handler, "/FriendSearchDefinitelyMissing", session+`{}`)
	var payload map[string]any
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &payload) != nil ||
		payload["res_code"] != float64(-1) || !strings.Contains(response.Body.String(), degrade.Message) {
		t.Fatalf("want degrader, got %s", response.Body.String())
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decodeCNPhoneSafe(t *testing.T, response *httptest.ResponseRecorder) (map[string]any, map[string]any) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(response.Body.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("want CN protocol lines, got %q", response.Body.String())
	}
	var common, method map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &common); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &method); err != nil {
		t.Fatal(err)
	}
	return common, method
}
