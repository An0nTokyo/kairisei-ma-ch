package degrade

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandlerReturnsFixedRefusalAndCreatesNoFile(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "account.sqlite3")
	request := httptest.NewRequest(http.MethodPost, "/FriendSearch", nil)
	request.Header.Set("X-Data-Dir", dir)
	response := httptest.NewRecorder()

	Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["res_code"] != float64(-1) || payload["res_str"] != Message {
		t.Fatalf("payload = %#v", payload)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("degrader created %s", marker)
	}
}
