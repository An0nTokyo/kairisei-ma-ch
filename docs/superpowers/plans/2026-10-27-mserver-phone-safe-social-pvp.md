# mserver phone-safe social/PVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace degrader (`res_code=-1`) for nine social/PVP POSTs on the phone with phone-safe stubs/wrappers so the CN client does not return to the login page.

**Architecture:** Add `registerPhoneSafeRoutes` in `cnbootstrap`, called from `NewSoloPlay`, so paths enter the play allow-list. Stubs never call mutating `httpapi` friend/PVP writers. Wrappers may `forwardCNBusiness` to existing handlers; on failure or missing defense they return non-`-1` shells. Remove `PvpStart2` from `registerDefenseRoutes` to avoid duplicate chi registration.

**Tech Stack:** Go 1.25, chi, existing `cnbootstrap` CN protocol helpers (`requireCNSessionOnly`, `writeCNProtocolResponseWithPopup`, `forwardCNBusiness`), `mserver` `server.NewHandler` + `localcn.OpenPlayFromDir` integration tests.

## Global Constraints

- Do not modify `server/internal/httpapi` friend/PVP function bodies.
- Do not change generic `degrade.Handler` semantics (`res_code=-1`, message `该功能需要连接电脑上的服务器`).
- Success paths must never return degrader `-1`.
- No real friend persistence; FollowAdd/UnFollow/PvpEnd must not change save revision / follow graph / PVP points.
- `PvpStart2` without usable defense/opponents returns `res_code=-1200`, never degrader.
- Scope is only the nine POSTs in the spec; no static `/disabled/web*` work.
- Spec: `docs/superpowers/specs/2026-10-27-mserver-phone-safe-social-pvp-design.md`.

---

## File map

| File | Responsibility |
|------|----------------|
| `server/internal/cnbootstrap/phone_safe_routes.go` | `registerPhoneSafeRoutes` + stub/wrapper handlers |
| `server/internal/cnbootstrap/phone_safe_routes_test.go` | Unit tests for stub JSON shapes (optional thin tests) |
| `server/internal/cnbootstrap/solo_play.go` | Call `registerPhoneSafeRoutes`; keep defense routes without `PvpStart2` |
| `server/internal/cnbootstrap/solo_routes.go` | Remove `PvpStart2` from `registerDefenseRoutes` |
| `mserver/internal/solo/phone_safe_test.go` | End-to-end: nine paths not degraded; no-op side effects; `-1200`; unknown path still degrades |
| `mserver/internal/solo/defense_test.go` | Keep AIRoom/Pvp regressions aligned with always-on `PvpStart2` wrapper |

---

### Task 1: Empty-list stubs + route registration

**Files:**
- Create: `server/internal/cnbootstrap/phone_safe_routes.go`
- Modify: `server/internal/cnbootstrap/solo_play.go` (append phone-safe paths after multi-room)
- Test: `mserver/internal/solo/phone_safe_test.go`

**Interfaces:**
- Consumes: `application`, `requireCNSessionOnly`, `normalizeCNOptionalFields` / `requireCNExactFields`, `writeCNProtocolResponseWithPopup`, `cnBootstrapCommon`
- Produces: `func (app *application) registerPhoneSafeRoutes(router chi.Router) []string`

- [ ] **Step 1: Write the failing integration test (lists not degraded)**

Create `mserver/internal/solo/phone_safe_test.go`:

```go
package solo_test

import (
	"encoding/json"
	"net/http"
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
```

Add `"net/http/httptest"` to the import block.

- [ ] **Step 2: Run test to verify it fails**

Run (from `mserver`):

```bash
go test ./internal/solo -count=1 -run TestPhoneSafeFriendListsAreEmptyNotDegraded
```

Expected: FAIL — body contains degrader message or missing fields.

- [ ] **Step 3: Implement stubs + registration**

Create `server/internal/cnbootstrap/phone_safe_routes.go`:

```go
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
	// Tasks 2–4 append more posts here (or register all nine in one file as you implement).
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
```

In `solo_play.go`, after multi-room registration:

```go
paths = append(paths, app.registerPhoneSafeRoutes(router)...)
```

Keep `registerDefenseRoutes` for now (Task 4 removes `PvpStart2` from it).

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/solo -count=1 -run TestPhoneSafeFriendListsAreEmptyNotDegraded
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/cnbootstrap/phone_safe_routes.go server/internal/cnbootstrap/solo_play.go mserver/internal/solo/phone_safe_test.go
git commit -m "Add phone-safe empty friend list stubs on mserver play routes."
```

---

### Task 2: Success no-op stubs (FollowAdd, FollowUnFollow, PvpEnd)

**Files:**
- Modify: `server/internal/cnbootstrap/phone_safe_routes.go`
- Modify: `mserver/internal/solo/phone_safe_test.go`

**Interfaces:**
- Consumes: `app.accounts` for PvpEnd read-only point/challenge (via session user id from context)
- Produces: handlers registered on `/FollowAdd`, `/FollowUnFollow`, `/PvpEnd`

- [ ] **Step 1: Write the failing test**

Append to `phone_safe_test.go`:

```go
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

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
```

Adjust `input_cmd` if native validation in a future forward path requires a real opcode; for pure stub, empty string in a one-element array is enough if stub only checks presence/types.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/solo -count=1 -run TestPhoneSafeFollowAndPvpEndDoNotMutate
```

Expected: FAIL (degraded).

- [ ] **Step 3: Implement no-op handlers**

In `phone_safe_routes.go`, register and implement:

```go
post("/FollowAdd", phoneSafeFollowAdd)
post("/FollowUnFollow", phoneSafeFollowUnfollow)
post("/PvpEnd", app.phoneSafePvpEnd)

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
```

Confirm `cnAuthenticatedUserKey` and `LoadState` / `state.PVP.Challenge` field names against `accountstore` / `gamestate` (use the same fields `PvpStatus` would expose). If `Challenge` lives elsewhere, read via the same path Home/PvpShow uses on state.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/solo -count=1 -run 'TestPhoneSafeFriendLists|TestPhoneSafeFollowAndPvpEnd'
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/cnbootstrap/phone_safe_routes.go mserver/internal/solo/phone_safe_test.go
git commit -m "Add phone-safe no-op Follow and PvpEnd stubs."
```

---

### Task 3: UserProfileShow and PvpShow wrappers

**Files:**
- Modify: `server/internal/cnbootstrap/phone_safe_routes.go`
- Modify: `mserver/internal/solo/phone_safe_test.go`

**Interfaces:**
- Consumes: `app.business`, `forwardCNBusiness`, `adaptCNBusinessRequest`, `cnBootstrapUserProfileShow` patterns / `cnBootstrapSessionOnlyBusiness` for PvpShow
- Produces: `/UserProfileShow`, `/PvpShow` on phone-safe router

- [ ] **Step 1: Write the failing test**

```go
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
```

Add `"fmt"` import.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/solo -count=1 -run TestPhoneSafeProfileAndPvpShowNotDegraded
```

Expected: FAIL (degraded).

- [ ] **Step 3: Implement wrappers**

Register:

```go
post("/UserProfileShow", app.phoneSafeUserProfileShow)
post("/PvpShow", app.phoneSafePvpShow)
```

Implementation sketch:

```go
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
	if recorder.Code == http.StatusOK && !bytes.Contains(recorder.Body.Bytes(), []byte(`"res_code":-1`)) {
		// Copy successful forward (including multi-line CN body).
		for k, vv := range recorder.Header() {
			for _, v := range vv {
				writer.Header().Add(k, v)
			}
		}
		writer.WriteHeader(recorder.Code)
		_, _ = writer.Write(recorder.Body.Bytes())
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
	if recorder.Code == http.StatusOK && !bytes.Contains(recorder.Body.Bytes(), []byte(degradeMessageIfAny)) {
		// Prefer checking HTTP 200 and parse res_code == 0 from line 0.
		lines := bytes.Split(bytes.TrimSpace(recorder.Body.Bytes()), []byte{'\n'})
		var common map[string]any
		if len(lines) >= 1 && json.Unmarshal(lines[0], &common) == nil {
			if code, _ := common["res_code"].(float64); code == 0 {
				for k, vv := range recorder.Header() {
					for _, v := range vv {
						writer.Header().Add(k, v)
					}
				}
				writer.WriteHeader(recorder.Code)
				_, _ = writer.Write(recorder.Body.Bytes())
				return
			}
		}
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
```

Do not import `mserver/degrade` from `cnbootstrap`. Detect failure via non-200 or `res_code != 0` on the forwarded recorder only.

Refine copy-forward helper into a small private `writeRecorderOrFallback` if duplication hurts.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/solo -count=1 -run TestPhoneSafeProfileAndPvpShowNotDegraded
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/cnbootstrap/phone_safe_routes.go mserver/internal/solo/phone_safe_test.go
git commit -m "Wrap UserProfileShow and PvpShow with phone-safe fallbacks."
```

---

### Task 4: Always-on PvpStart2 wrapper; remove defense duplicate

**Files:**
- Modify: `server/internal/cnbootstrap/phone_safe_routes.go`
- Modify: `server/internal/cnbootstrap/solo_routes.go` (`registerDefenseRoutes` — remove `PvpStart2` only)
- Modify: `mserver/internal/solo/phone_safe_test.go`
- Modify: `mserver/internal/solo/defense_test.go` if assertions assume defense-only mount for PvpStart2

**Interfaces:**
- Consumes: `app.business`, existing `cnBootstrapExactBusiness` field list for PvpStart2
- Produces: single `/PvpStart2` registration on phone-safe routes

- [ ] **Step 1: Write the failing tests**

```go
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
```

Keep `TestPvpUsesBorrowedDefenseDecks` — it must still pass after moving mount to phone-safe wrapper that forwards when defense exists.

- [ ] **Step 2: Run new tests to verify failure mode**

```bash
go test ./internal/solo -count=1 -run 'TestPhoneSafePvpStartWithoutDefense|TestUnknownRouteStillDegraded'
```

Expected: `TestPhoneSafePvpStartWithoutDefense` FAIL (degraded `-1`); `TestUnknownRouteStillDegraded` PASS already.

- [ ] **Step 3: Implement PvpStart2 wrapper; un-duplicate**

In `solo_routes.go` `registerDefenseRoutes`, delete the `PvpStart2` post (leave empty function returning nil paths, or delete the whole defense route if it only had PvpStart2 — today it only registers `PvpStart2`, so make `registerDefenseRoutes` return `nil` / empty and stop calling it from `solo_play.go` **or** keep the call for future hooks but with no routes).

Simplest per spec: remove `PvpStart2` registration; if `registerDefenseRoutes` becomes empty, remove the `if opponents.ready { append defense }` block from `solo_play.go` (defense decks still load into `frozenPVPOpponents` for the business builder).

Phone-safe handler:

```go
post("/PvpStart2", app.phoneSafePvpStart2)

func (app *application) phoneSafePvpStart2(writer http.ResponseWriter, request *http.Request) {
	payload, err := normalizeCNOptionalFields(request, "PvpStart2",
		[]string{"type", "select_arthur_type", "pvp_my_deck"},
		[]string{"type", "select_arthur_type", "pvp_my_deck"})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	recorder := httptest.NewRecorder()
	forwardCNBusiness(
		recorder,
		adaptCNBusinessRequest(request, http.MethodPost, "/PvpStart2", payload),
		app.business,
		func(content []byte) ([]byte, error) { return content, nil },
	)
	lines := bytes.Split(bytes.TrimSpace(recorder.Body.Bytes()), []byte{'\n'})
	var common map[string]any
	if recorder.Code == http.StatusOK && len(lines) >= 1 && json.Unmarshal(lines[0], &common) == nil {
		code, _ := common["res_code"].(float64)
		// Forward success or business -1200 from real handler.
		if code == 0 || code == -1200 {
			for k, vv := range recorder.Header() {
				for _, v := range vv {
					writer.Header().Add(k, v)
				}
			}
			writer.WriteHeader(recorder.Code)
			_, _ = writer.Write(recorder.Body.Bytes())
			return
		}
	}
	commonOut := cnBootstrapCommon()
	commonOut["res_code"] = -1200
	commonOut["res_str"] = "暂无可用的本地竞技场对手，请在其他玩家建立卡组后重试。"
	writeCNProtocolResponseWithPopup(writer, commonOut, map[string]any{})
}
```

- [ ] **Step 4: Run solo package tests**

```bash
go test ./internal/solo -count=1
go test ./internal/cnbootstrap -count=1 -run PhoneSafe
```

Expected: all PASS (fix `cnbootstrap` phone-safe tests if added; otherwise solo package green including defense/PVP regressions).

- [ ] **Step 5: Commit**

```bash
git add server/internal/cnbootstrap/phone_safe_routes.go server/internal/cnbootstrap/solo_routes.go server/internal/cnbootstrap/solo_play.go mserver/internal/solo/phone_safe_test.go mserver/internal/solo/defense_test.go
git commit -m "Always mount phone-safe PvpStart2 with soft -1200 without defense."
```

---

## Spec coverage self-check

| Spec requirement | Task |
|------------------|------|
| Empty Follow/Follower/FriendSearch | Task 1 |
| No-op FollowAdd/UnFollow/PvpEnd, no mutate | Task 2 |
| UserProfileShow / PvpShow forward + fallback | Task 3 |
| PvpStart2 always on, `-1200`, remove defense dup | Task 4 |
| Other routes still degrade | Task 4 `TestUnknownRouteStillDegraded` |
| No httpapi body edits / no degrader semantic change | All tasks |

## Placeholder scan

No TBD/TODO left in steps; field names for `state.PVP.Challenge` must be verified against `gamestate` at implement time (use whatever `PvpStatus` reads).
