package solo_test

import (
	"encoding/json"
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
