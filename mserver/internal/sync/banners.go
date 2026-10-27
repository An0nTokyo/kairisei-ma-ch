package sync

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type BannerFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  string `json:"bytes"`
}

func FetchBanners(baseURL, token string) ([]BannerFile, int, error) {
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/local/sync/banners", nil)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("X-Account-Sync-Token", token)
	client := &http.Client{Timeout: 2 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return nil, 0, err
	}
	if response.StatusCode != http.StatusOK {
		var decoded struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(payload, &decoded) == nil && decoded.Error != "" {
			return nil, 0, errors.New(decoded.Error)
		}
		snippet := strings.TrimSpace(string(payload))
		if len(snippet) > 120 {
			snippet = snippet[:120]
		}
		if snippet == "" {
			return nil, 0, errors.New("banner sync failed: HTTP " + strconv.Itoa(response.StatusCode))
		}
		return nil, 0, errors.New("banner sync failed: HTTP " + strconv.Itoa(response.StatusCode) + " " + snippet)
	}
	var body struct {
		Files   []BannerFile `json:"files"`
		Missing int          `json:"missing"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, 0, err
	}
	if body.Files == nil {
		body.Files = []BannerFile{}
	}
	return body.Files, body.Missing, nil
}
