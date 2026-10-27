package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestLauncherBannersWritesLocalFiles(t *testing.T) {
	dir := t.TempDir()
	png := defaultBannerPNG()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/local/sync/banners" || r.Header.Get("X-Account-Sync-Token") != "secret" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"missing": 0,
			"files": []map[string]string{{
				"path":   "/local/gacha/banner.png",
				"sha256": "x",
				"bytes":  base64.StdEncoding.EncodeToString(png),
			}},
		})
	}))
	t.Cleanup(upstream.Close)
	u, _ := url.Parse(upstream.URL)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)

	handler := NewHandler(Deps{DataDir: dir, DeviceID: "dev", GamePort: 26020})
	body := fmt.Sprintf(`{"host":%q,"port":%d,"token":"secret"}`, host, port)
	req := httptest.NewRequest(http.MethodPost, "/local/launcher/banners", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	stored := filepath.Join(bannerRoot(dir), "local", "gacha", "banner.png")
	got, err := os.ReadFile(stored)
	if err != nil || !bytes.Equal(got, png) {
		t.Fatalf("stored err=%v", err)
	}
}
