package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"kairisei.local/mserver/internal/addresses"
	"kairisei.local/mserver/internal/degrade"
	"kairisei.local/mserver/internal/deviceid"
	"kairisei.local/server/localcn"
)

func TestLoginUsesSocketAddress(t *testing.T) {
	dir := t.TempDir()
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
	lister := func() ([]addresses.Interface, error) {
		return nil, errLister
	}
	handler := NewHandler(Deps{
		Accounts:   store,
		DeviceID:   device,
		GamePort:   26020,
		Interfaces: func() ([]addresses.Interface, error) { return lister() },
	})

	success := loginResponse(t, handler, "00000000-0000-0000-0000-000000000099")
	if success["api_url"] != "http://192.0.2.8:26020/" {
		t.Fatalf("api_url = %#v", success["api_url"])
	}
	for _, key := range []string{"version_url", "res_patch_url", "res_cpk_url", "res_img_url"} {
		text, _ := success[key].(string)
		if !strings.Contains(text, "192.0.2.8") || strings.Contains(text, "evil.example") {
			t.Fatalf("%s = %#v", key, success[key])
		}
	}
	if success["sess_key"] == "held-session" {
		t.Fatal("session was not rotated")
	}
	rotated := sessionOf(t, store)

	unknown := loginResponse(t, handler, "00000000-0000-0000-0000-000000000088")
	if unknown["res_code"] != float64(-1) || unknown["res_str"] != "该账号不在这台手机上" {
		t.Fatalf("unknown = %#v", unknown)
	}
	if players := playerCount(t, store); players != 1 {
		t.Fatalf("player rows = %d", players)
	}
	if sessionOf(t, store) != rotated {
		t.Fatal("unknown login changed the session")
	}

	if err := store.PutHolder(1000002, "other-phone"); err != nil {
		t.Fatal(err)
	}
	refused := loginResponse(t, handler, "00000000-0000-0000-0000-000000000099")
	if refused["res_code"] != float64(-1) || refused["res_str"] != "该账号不在这台手机上" {
		t.Fatalf("other holder = %#v", refused)
	}
	if sessionOf(t, store) != rotated {
		t.Fatal("refused login changed the session")
	}

	list := requestText(t, handler, http.MethodGet, "/local/server/default.list", "")
	if !strings.Contains(list, "192.0.2.8") || !strings.Contains(list, "26020") || strings.Contains(list, "evil.example") {
		t.Fatalf("server list = %s", list)
	}

	empty := requestText(t, handler, http.MethodGet, "/local/launcher/addresses", "")
	if strings.TrimSpace(empty) != "[]" {
		t.Fatalf("failed address list = %s", empty)
	}
	lister = func() ([]addresses.Interface, error) {
		return []addresses.Interface{
			{Name: "lo", Up: true, Addrs: netIP("127.0.0.1", "127.0.0.2")},
			{Name: "wlan0", Up: true, Addrs: netIP("192.168.1.23")},
		}, nil
	}
	listed := requestText(t, handler, http.MethodGet, "/local/launcher/addresses", "")
	if !strings.Contains(listed, "本机 · lo") || !strings.Contains(listed, "192.168.1.23") {
		t.Fatalf("addresses = %s", listed)
	}

	for _, call := range []struct{ method, path string }{
		{http.MethodPost, "/local/version/resources/Android/patch/catalog.dat"},
		{http.MethodGet, "/local/resources/cpk/example.cpk"},
	} {
		body := requestText(t, handler, call.method, call.path, "")
		if !strings.Contains(body, degrade.Message) {
			t.Fatalf("%s %s = %s", call.method, call.path, body)
		}
	}
	if players := playerCount(t, store); players != 1 {
		t.Fatalf("player rows after degrade = %d", players)
	}
}

var errLister = errString("down")

type errString string

func (err errString) Error() string { return string(err) }

func loginResponse(t *testing.T, handler http.Handler, uuid string) map[string]any {
	t.Helper()
	body := requestText(t, handler, http.MethodPost, "/loginSDK.php", `{"uuid":"`+uuid+`","clver":"6.0.7"}`)
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func requestText(t *testing.T, handler http.Handler, method, path, body string) string {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Host = "evil.example:9999"
	request = request.WithContext(WithLocalAddr(request.Context(), "192.0.2.8"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d body %s", method, path, response.Code, response.Body.String())
	}
	return response.Body.String()
}

func netIP(values ...string) []net.IP {
	ips := make([]net.IP, 0, len(values))
	for _, value := range values {
		ips = append(ips, net.ParseIP(value))
	}
	return ips
}

func playerCount(t *testing.T, store *localcn.Store) int {
	t.Helper()
	count, err := store.PlayerCount()
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func sessionOf(t *testing.T, store *localcn.Store) string {
	t.Helper()
	session, err := store.SessionKey(1000002)
	if err != nil {
		t.Fatal(err)
	}
	return session
}
