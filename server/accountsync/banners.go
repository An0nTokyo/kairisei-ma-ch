package accountsync

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"os"
)

type BannerSource struct {
	URLPath string
	File    string
}

type BannerFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  string `json:"bytes"`
}

func BuildBannerFiles(sources []BannerSource) ([]BannerFile, int) {
	var files []BannerFile
	missing := 0
	for _, source := range sources {
		body, err := os.ReadFile(source.File)
		if err != nil {
			missing++
			continue
		}
		sum := sha256.Sum256(body)
		files = append(files, BannerFile{
			Path:   source.URLPath,
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  base64.StdEncoding.EncodeToString(body),
		})
	}
	return files, missing
}

func BannersHandler(token string, files []BannerFile, missing int) http.Handler {
	if files == nil {
		files = []BannerFile{}
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || token == "" ||
			subtle.ConstantTimeCompare([]byte(request.Header.Get(tokenHeader)), []byte(token)) != 1 {
			http.NotFound(writer, request)
			return
		}
		writeJSON(writer, http.StatusOK, map[string]any{"files": files, "missing": missing})
	})
}
