package accountsync_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"kairisei.local/server/accountsync"
)

func TestBannersHandlerRequiresTokenAndReturnsFiles(t *testing.T) {
	dir := t.TempDir()
	png := mustDecodePNG(t)
	path := filepath.Join(dir, "banner.png")
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(png)
	files, missing := accountsync.BuildBannerFiles([]accountsync.BannerSource{
		{URLPath: "/local/home/banner.png", File: path},
		{URLPath: "/local/gacha/missing.png", File: filepath.Join(dir, "nope.png")},
	})
	if missing != 1 || len(files) != 1 || files[0].Path != "/local/home/banner.png" {
		t.Fatalf("files=%v missing=%d", files, missing)
	}
	handler := accountsync.BannersHandler("secret", files, missing)

	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/local/sync/banners", nil))
	if denied.Code != http.StatusNotFound {
		t.Fatalf("no token status=%d", denied.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/local/sync/banners", nil)
	request.Header.Set("X-Account-Sync-Token", "secret")
	ok := httptest.NewRecorder()
	handler.ServeHTTP(ok, request)
	if ok.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", ok.Code, ok.Body.String())
	}
	var body struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Bytes  string `json:"bytes"`
		} `json:"files"`
		Missing int `json:"missing"`
	}
	if err := json.Unmarshal(ok.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Missing != 1 || len(body.Files) != 1 || body.Files[0].Path != "/local/home/banner.png" {
		t.Fatalf("%+v", body)
	}
	if body.Files[0].SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha=%s", body.Files[0].SHA256)
	}
	got, err := base64.StdEncoding.DecodeString(body.Files[0].Bytes)
	if err != nil || string(got) != string(png) {
		t.Fatalf("bytes mismatch err=%v", err)
	}
}

func mustDecodePNG(t *testing.T) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
