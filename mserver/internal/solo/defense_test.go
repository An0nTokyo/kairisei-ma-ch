package solo_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kairisei.local/mserver/internal/degrade"
	"kairisei.local/mserver/internal/server"
	"kairisei.local/mserver/internal/solo"
	"kairisei.local/server/localcn"
	"kairisei.local/server/playtest"
)

func TestPvpWithoutDefenseDecksIsDegraded(t *testing.T) {
	store, userID, handler := openDefensePhone(t, false)
	before, err := store.SnapshotRevision(userID)
	if err != nil {
		t.Fatal(err)
	}
	session := phoneSession(t, store, userID)
	for _, path := range []string{"/PvpStart2", "/TeamBattleAIRoomCreate"} {
		response := postPhone(handler, path, session+`{}`)
		var payload map[string]any
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &payload) != nil || payload["res_code"] != float64(-1) || !strings.Contains(response.Body.String(), degrade.Message) {
			t.Fatalf("%s status = %d body = %s", path, response.Code, response.Body.String())
		}
	}
	after, err := store.SnapshotRevision(userID)
	if err != nil || after != before {
		t.Fatalf("revision after degraded arena = %d want %d err %v", after, before, err)
	}
}

func TestPvpUsesBorrowedDefenseDecks(t *testing.T) {
	store, userID, handler := openDefensePhone(t, true)
	session := phoneSession(t, store, userID)
	body, err := json.Marshal(map[string]any{
		"type":               0,
		"select_arthur_type": 1,
		"pvp_my_deck":        ownPVPDecks(t, store, userID),
	})
	if err != nil {
		t.Fatal(err)
	}
	response := postPhone(handler, "/PvpStart2", session+string(body))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), degrade.Message) || !strings.Contains(response.Body.String(), `"cardid":4242`) {
		t.Fatalf("pvp status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestAIRoomServedWhenDefenseDecksWereBorrowed(t *testing.T) {
	store, userID, handler := openDefensePhone(t, true)
	session := phoneSession(t, store, userID)
	response := postPhone(handler, "/TeamBattleAIRoomCreate", session+`{"bossid":1,"ai_id":1,"deck_arthur_type":1,"deck_arthur_type_idx":0,"use_punished_free_piont":0}`)
	if strings.Contains(response.Body.String(), degrade.Message) || !strings.Contains(response.Body.String(), "unknown local AI room owner") {
		t.Fatalf("ai room status = %d body = %s", response.Code, response.Body.String())
	}
}

func openDefensePhone(t *testing.T, withDecks bool) (*localcn.Store, int, http.Handler) {
	t.Helper()
	root := t.TempDir()
	configDir := filepath.Join("..", "..", "..", "server", "config")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := localcn.Open(filepath.Join(root, "save.json"), playtest.Save(t, root, configDir), logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.LoadOrImport(); err != nil {
		t.Fatal(err)
	}
	const deviceID = "device-phone"
	userID, err := store.ResolveLogin("00000000-0000-0000-0000-000000000099")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutHolder(userID, deviceID); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(wd, "..", "..", "..", "server", "internal", "cnbootstrap")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	options := playtest.Write(t, root, filepath.Join("..", "..", "config"))
	stagePhoneMasters(t, root, options, filepath.Join(wd, "..", "..", "..", "server", "config", "cn602-pvp-runtime.json"))
	if withDecks {
		if err := os.WriteFile(solo.FilePath(root), []byte(frozenDefenseDecks), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	play, err := localcn.OpenPlayFromDir(store, root, deviceID, "http://127.0.0.1:26020", "127.0.0.1", 26021)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.NewHandler(server.Deps{
		Accounts: store,
		DeviceID: deviceID,
		DataDir:  root,
		GamePort: 26020,
		Play:     play,
	})
	return store, userID, handler
}

func stagePhoneMasters(t *testing.T, root string, options localcn.PlayOptions, pvpPath string) {
	t.Helper()
	masters := filepath.Join(root, "masters")
	if err := os.MkdirAll(masters, 0o700); err != nil {
		t.Fatal(err)
	}
	copyMaster := func(src, name string) {
		t.Helper()
		if src == "" {
			return
		}
		body, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(masters, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	copyMaster(options.Cards, "cards.bin")
	copyMaster(options.Explore, "explore.bin")
	copyMaster(options.Story, "story.bin")
	copyMaster(options.Navi, "navi.bin")
	copyMaster(options.Items, "items.bin")
	copyMaster(options.Avatar, "avatar.bin")
	copyMaster(options.Stamps, "stamps.bin")
	copyMaster(options.Honors, "honors.bin")
	copyMaster(options.PlayerProgression, "growth.bin")
	copyMaster(options.LoginBonus, "login-bonus.bin")
	copyMaster(pvpPath, "pvp.bin")
}

func phoneSession(t *testing.T, store *localcn.Store, userID int) string {
	t.Helper()
	session, err := store.SessionKey(userID)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func postPhone(handler http.Handler, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return response
}

func ownPVPDecks(t *testing.T, store *localcn.Store, userID int) []map[string]any {
	t.Helper()
	raw, err := store.ExportTransferJSON(userID)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Account struct {
			Decks []struct {
				ArthurType int     `json:"arthur_type"`
				JobType    int     `json:"job_type"`
				Index      int     `json:"index"`
				IsActive   int     `json:"is_active"`
				Leader     int     `json:"leader_card_index"`
				Cards      []int64 `json:"card_unique_ids"`
				Support    []int64 `json:"support_card_unique_ids"`
				Spheres    []int64 `json:"sphere_unique_ids"`
				Buddies    []int64 `json:"buddy_unique_ids"`
			} `json:"decks"`
		} `json:"Account"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	chosen := map[int]int{}
	for index, deck := range document.Account.Decks {
		current, exists := chosen[deck.ArthurType]
		if !exists || (deck.IsActive != 0 && document.Account.Decks[current].IsActive == 0) ||
			(deck.IsActive == document.Account.Decks[current].IsActive && deck.Index < document.Account.Decks[current].Index) {
			chosen[deck.ArthurType] = index
		}
	}
	decks := make([]map[string]any, 0, 4)
	for arthur := 1; arthur <= 4; arthur++ {
		index, exists := chosen[arthur]
		if !exists {
			t.Fatalf("borrowed account has no deck for Arthur %d", arthur)
		}
		deck := document.Account.Decks[index]
		decks = append(decks, map[string]any{
			"arthur_type":         deck.ArthurType,
			"job_type":            deck.JobType,
			"deck_idx":            deck.Index,
			"leader_card_idx":     deck.Leader,
			"card_uniqid":         deck.Cards,
			"support_card_uniqid": deck.Support,
			"sphr_uniqid":         deck.Spheres,
			"buddy_uniqid":        deck.Buddies,
		})
	}
	return decks
}

const frozenDefenseDecks = `[{
  "user": {"user_id": 1000002, "level": 10, "jobs": [{"hp":10,"atkp":10,"intp":10,"mndp":10},{"hp":10,"atkp":10,"intp":10,"mndp":10},{"hp":10,"atkp":10,"intp":10,"mndp":10},{"hp":10,"atkp":10,"intp":10,"mndp":10},{"hp":10,"atkp":10,"intp":10,"mndp":10}]},
  "cards": [{"unique_id": 7, "card_id": 4242, "level": 1, "hp": 100, "attack": 100, "magic": 100, "mind": 100}],
  "pvp": {"next_battle_id": 1, "history": [], "defense_decks": [
    {"arthur_type": 1, "job_type": 1, "leader_card_idx": 0, "card_uniqid": [7], "support_card_uniqid": [], "sphr_uniqid": [], "buddy_uniqid": []},
    {"arthur_type": 2, "job_type": 2, "leader_card_idx": 0, "card_uniqid": [7], "support_card_uniqid": [], "sphr_uniqid": [], "buddy_uniqid": []},
    {"arthur_type": 3, "job_type": 3, "leader_card_idx": 0, "card_uniqid": [7], "support_card_uniqid": [], "sphr_uniqid": [], "buddy_uniqid": []},
    {"arthur_type": 4, "job_type": 4, "leader_card_idx": 0, "card_uniqid": [7], "support_card_uniqid": [], "sphr_uniqid": [], "buddy_uniqid": []}
  ]}
}]`
