package server

import (
	_ "embed"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed default_banner.png
var defaultBannerPNGBytes []byte

func defaultBannerPNG() []byte { return defaultBannerPNGBytes }

func bannerRoot(dataDir string) string { return filepath.Join(dataDir, "banners") }

func serveLocalBanners(root string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rel, err := bannerURLToRel(request.URL.Path)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		body := defaultBannerPNG()
		if raw, err := os.ReadFile(path); err == nil && len(raw) >= 8 && string(raw[:8]) == "\x89PNG\r\n\x1a\n" {
			body = raw
		}
		writer.Header().Set("Content-Type", "image/png")
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
		writer.WriteHeader(http.StatusOK)
		if request.Method != http.MethodHead {
			_, _ = writer.Write(body)
		}
	})
}

func bannerURLToRel(urlPath string) (string, error) {
	if strings.Contains(urlPath, "..") {
		return "", errors.New("invalid banner path")
	}
	switch {
	case strings.HasPrefix(urlPath, "/local/home/"):
		rest := strings.TrimPrefix(urlPath, "/local/home/")
		if rest == "" || strings.Contains(rest, "..") {
			return "", errors.New("invalid banner path")
		}
		return "local/home/" + rest, nil
	case strings.HasPrefix(urlPath, "/local/gacha/"):
		rest := strings.TrimPrefix(urlPath, "/local/gacha/")
		if rest == "" || strings.Contains(rest, "..") {
			return "", errors.New("invalid banner path")
		}
		return "local/gacha/" + rest, nil
	default:
		return "", errors.New("invalid banner path")
	}
}

type bannerSyncFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  string `json:"bytes"`
}

func writeBannerFiles(root string, files []bannerSyncFile) (int, error) {
	synced := 0
	for _, file := range files {
		rel, err := bannerURLToRel(file.Path)
		if err != nil {
			return synced, err
		}
		raw, err := base64.StdEncoding.DecodeString(file.Bytes)
		if err != nil || len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return synced, err
		}
		tmp := target + ".tmp"
		if err := os.WriteFile(tmp, raw, 0o600); err != nil {
			return synced, err
		}
		if err := os.Rename(tmp, target); err != nil {
			return synced, err
		}
		synced++
	}
	return synced, nil
}
