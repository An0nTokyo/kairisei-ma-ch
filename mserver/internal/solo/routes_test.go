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
	"kairisei.local/server/localcn"
	"kairisei.local/server/playtest"
)

func TestSoloRouteWritesAndFriendRouteDoesNot(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join("..", "..", "..", "server", "config")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := localcn.Open(filepath.Join(root, "phone.json"), playtest.Save(t, root, configDir), logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.LoadOrImport(); err != nil {
		t.Fatal(err)
	}
	const deviceID = "device-phone"
	const loginUUID = "00000000-0000-0000-0000-000000000099"
	userID, err := store.ResolveLogin(loginUUID)
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
	options.DeviceID = deviceID
	options.BaseURL = "http://127.0.0.1:26020"
	options.Logger = logger
	options.BattleHost = "127.0.0.1"
	options.BattlePort = 26021
	play, err := localcn.OpenPlay(store, options)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.NewHandler(server.Deps{
		Accounts: store,
		DeviceID: deviceID,
		GamePort: 26020,
		Play:     play,
	})
	before, err := store.SnapshotRevision(userID)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.SessionKey(userID)
	if err != nil {
		t.Fatal(err)
	}
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest(http.MethodPost, "/HomeShow", strings.NewReader(session)))
	if home.Code != http.StatusOK {
		t.Fatalf("home status = %d body = %s", home.Code, home.Body.String())
	}
	after, err := store.SnapshotRevision(userID)
	if err != nil || after <= before {
		t.Fatalf("revision after home = %d before %d err %v body %s", after, before, err, home.Body.String())
	}
	friend := httptest.NewRecorder()
	handler.ServeHTTP(friend, httptest.NewRequest(http.MethodPost, "/FriendSearch", strings.NewReader(session+`{}`)))
	var payload map[string]any
	if friend.Code != http.StatusOK || json.Unmarshal(friend.Body.Bytes(), &payload) != nil || payload["res_code"] != float64(-1) || !strings.Contains(friend.Body.String(), degrade.Message) {
		t.Fatalf("friend status = %d body = %s", friend.Code, friend.Body.String())
	}
	stayed, err := store.SnapshotRevision(userID)
	if err != nil || stayed != after {
		t.Fatalf("revision after friend = %d want %d err %v", stayed, after, err)
	}
}
