package server

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHomeBannerServesLocalFileOrPlaceholder(t *testing.T) {
	dir := t.TempDir()
	root := bannerRoot(dir)
	handler := serveLocalBanners(root)

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/local/home/banner.png", nil))
	if missing.Code != http.StatusOK || missing.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("placeholder status=%d type=%s", missing.Code, missing.Header().Get("Content-Type"))
	}
	if got := missing.Body.Bytes(); len(got) < 8 || string(got[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("want PNG magic, got %x", got[:min(8, len(got))])
	}

	local := defaultBannerPNG()
	target := filepath.Join(root, "local", "home", "banner.png")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, local, 0o600); err != nil {
		t.Fatal(err)
	}
	hit := httptest.NewRecorder()
	handler.ServeHTTP(hit, httptest.NewRequest(http.MethodGet, "/local/home/banner.png", nil))
	if hit.Code != http.StatusOK || hit.Body.Len() != len(local) {
		t.Fatalf("local status=%d len=%d", hit.Code, hit.Body.Len())
	}
}

func TestRejectBannerPathTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := writeBannerFiles(bannerRoot(dir), []bannerSyncFile{{
		Path:  "/local/home/../../etc/passwd",
		Bytes: base64.StdEncoding.EncodeToString(defaultBannerPNG()),
	}})
	if err == nil {
		t.Fatal("expected rejection")
	}
}
