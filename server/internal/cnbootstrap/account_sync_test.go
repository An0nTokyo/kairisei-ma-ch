package cnbootstrap

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"kairisei.local/server/internal/accountstore"
)

func TestBorrowedLoginDoesNotRotateTheSession(t *testing.T) {
	directory := t.TempDir()
	storage, err := accountstore.OpenDatabase(
		filepath.Join(directory, "save.json"),
		filepath.Join("..", "..", "config", "cn602-save-template.json"),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	accounts, err := accountstore.NewAccounts(storage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.LoadOrImport(); err != nil {
		t.Fatal(err)
	}
	const loginUUID = "00000000-0000-0000-0000-000000000099"
	userID, err := accounts.ResolveLoginUser(loginUUID)
	if err != nil {
		t.Fatal(err)
	}
	database, err := accounts.OpenStorage()
	if err != nil {
		t.Fatal(err)
	}
	var before string
	if err := database.QueryRow(`SELECT session_key FROM cn_local_account WHERE user_id = ?`, userID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := accounts.PutHolder(accountstore.Holder{UserID: userID, HolderID: "phone"}); err != nil {
		t.Fatal(err)
	}
	handler := refuseAwayLogin(accounts, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("borrowed login continued")
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/loginSDK.php", strings.NewReader(`{"uuid":"`+loginUUID+`","clver":"6.0.7"}`)))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "该账号不在这台电脑上") {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var after string
	if err := database.QueryRow(`SELECT session_key FROM cn_local_account WHERE user_id = ?`, userID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("session changed from %s to %s", before, after)
	}
}
