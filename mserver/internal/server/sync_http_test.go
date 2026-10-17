package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLauncherAccountList(t *testing.T) {
	var lines []string
	previous := writeSyncLog
	writeSyncLog = func(line string) { lines = append(lines, line) }
	t.Cleanup(func() { writeSyncLog = previous })

	called := false
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		called = true
		if request.Header.Get("X-Account-Sync-Token") != "secret" {
			http.NotFound(writer, request)
			return
		}
		var payload struct {
			DeviceID string `json:"device_id"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.DeviceID != "device-a" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"accounts":[{"user_id":1000001,"name":"秋","state":"server"}]}`)
	}))
	t.Cleanup(remote.Close)
	handler := NewHandler(Deps{DeviceID: "device-a"})

	post := func(remoteAddr, body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/local/launcher/accounts", strings.NewReader(body))
		request.RemoteAddr = remoteAddr
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}
	away := post("192.0.2.8:9", `{"host":"127.0.0.1","port":1,"token":"secret"}`)
	if away.Code != http.StatusNotFound || called {
		t.Fatalf("non-loopback = %d called %v", away.Code, called)
	}
	for _, body := range []string{
		`{"host":"","port":26020,"token":"secret"}`,
		`{"host":"127.0.0.1","port":0,"token":"secret"}`,
		`{"host":"127.0.0.1","port":26020,"token":""}`,
	} {
		got := post("127.0.0.1:9", body)
		if got.Code != http.StatusBadRequest || !strings.Contains(got.Body.String(), "account sync body is invalid") || called {
			t.Fatalf("invalid %s = %d %s called %v", body, got.Code, got.Body.String(), called)
		}
	}

	computer, err := http.NewRequest(http.MethodGet, remote.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	host := computer.URL.Hostname()
	port := computer.URL.Port()
	missing := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	}))
	t.Cleanup(missing.Close)
	missingURL, err := http.NewRequest(http.MethodGet, missing.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	failed := post("127.0.0.1:9", `{"host":"`+missingURL.URL.Hostname()+`","port":`+missingURL.URL.Port()+`,"token":"secret"}`)
	if failed.Code != http.StatusBadRequest || !strings.Contains(failed.Body.String(), "account sync failed") {
		t.Fatalf("upstream 404 = %d %s", failed.Code, failed.Body.String())
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "账号列表失败: account sync failed") || strings.Contains(joined, "获取账号列表") {
		t.Fatalf("failure log = %s", joined)
	}

	lines = nil
	ok := post("127.0.0.1:9", `{"host":"`+host+`","port":`+port+`,"token":"secret"}`)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"name":"秋"`) {
		t.Fatalf("list = %d %s", ok.Code, ok.Body.String())
	}
	joined = strings.Join(lines, "\n")
	if !strings.Contains(joined, "获取账号列表") || !strings.Contains(joined, "账号列表 1 个") {
		t.Fatalf("success log = %s", joined)
	}
}
