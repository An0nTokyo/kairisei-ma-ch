package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kairisei.local/mserver/internal/degrade"
	"kairisei.local/mserver/internal/deviceid"
	"kairisei.local/server/localcn"
)

func TestResourceCatalogIsAdvertisedAndDownloadsStayRefused(t *testing.T) {
	dir := t.TempDir()
	catalog := []byte("catalog-bytes")
	cpkList := []byte("# cpk_name,type,id,file_size_mb,download_type,size_bytes\ncv_navi_1.cpk,NAVI_VOICE,1,1.000000,ALL,10\n")
	cpkState := []byte("cv_navi_1.cpk,3\n")
	resourceDir := filepath.Join(dir, "resources")
	if err := os.MkdirAll(resourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{
		"catalog.dat":    catalog,
		"cpk_file.csv":   cpkList,
		"cpk_patch.txt":  cpkState,
	} {
		if err := os.WriteFile(filepath.Join(resourceDir, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	device, err := deviceid.LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	store, err := localcn.Open(
		filepath.Join(dir, "save.json"),
		filepath.Join("..", "..", "..", "server", "config", "cn602-save-template.json"),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := store.InsertLogin(1000002, "00000000-0000-0000-0000-000000000099", "held-session"); err != nil {
		t.Fatal(err)
	}
	if err := store.PutHolder(1000002, device); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Deps{Accounts: store, DeviceID: device, DataDir: dir, GamePort: 26020})

	namespace := localcn.ResourceVersionNamespace(catalog, cpkList, cpkState)
	login := loginResponse(t, handler, "00000000-0000-0000-0000-000000000099")
	versionURL, _ := login["version_url"].(string)
	if !strings.Contains(versionURL, "/local/version/"+namespace+"/") {
		t.Fatalf("version_url = %#v", login["version_url"])
	}

	for _, item := range []struct {
		path, contentType string
		body              []byte
	}{
		{"/local/version/" + namespace + "/Android/patch/catalog.dat", "application/octet-stream", catalog},
		{"/local/version/" + namespace + "/CPK/cpk_file.csv", "text/plain; charset=utf-8", cpkList},
		{"/local/version/" + namespace + "/CPK/cpk_patch.txt", "text/plain; charset=utf-8", cpkState},
	} {
		request := httptest.NewRequest(http.MethodGet, item.path, nil)
		request = request.WithContext(WithLocalAddr(request.Context(), "192.0.2.8"))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Body.String() != string(item.body) || response.Header().Get("Content-Type") != item.contentType {
			t.Fatalf("%s status=%d type=%q body=%q", item.path, response.Code, response.Header().Get("Content-Type"), response.Body.String())
		}
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/local/resources/patch/Android/patch/main_c/scenes/scene_Menu.dat.v143C9C4D" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write([]byte("menu-bytes"))
	}))
	t.Cleanup(upstream.Close)
	if err := os.WriteFile(filepath.Join(dir, "resource-upstream"), []byte(upstream.URL), 0o600); err != nil {
		t.Fatal(err)
	}
	proxied := NewHandler(Deps{Accounts: store, DeviceID: device, DataDir: dir, GamePort: 26020})
	menu := requestText(t, proxied, http.MethodGet, "/local/resources/patch/Android/patch/main_c/scenes/scene_Menu.dat.v143C9C4D", "")
	if menu != "menu-bytes" {
		t.Fatalf("proxied menu = %q", menu)
	}

	for _, path := range []string{
		"/local/version/" + namespace + "/Android/patch/main_c/scenes/scene_Menu.dat",
		"/local/resources/patch/Android/patch/main_c/scenes/scene_Menu.dat.v143C9C4D",
		"/local/resources/cpk/cv_navi_1.cpk",
	} {
		body := requestText(t, handler, http.MethodGet, path, "")
		if !strings.Contains(body, degrade.Message) {
			t.Fatalf("%s = %s", path, body)
		}
	}
}
