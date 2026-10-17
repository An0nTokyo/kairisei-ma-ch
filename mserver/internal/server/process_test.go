package server

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestStartRejectsInvalidPortBeforeListen(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "account-marker.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"0", "65535"} {
		if _, err := Start(dir, text); err == nil || !strings.Contains(err.Error(), "本机端口无效") {
			t.Fatalf("Start(%s) error = %v", text, err)
		}
	}
	kept, err := os.ReadFile(marker)
	if err != nil || string(kept) != "keep" {
		t.Fatalf("marker = %q, %v", kept, err)
	}
}

func TestStartRejectsInvalidDeviceID(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "account-marker.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	idPath := filepath.Join(dir, "device-id")
	if err := os.WriteFile(idPath, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(dir, "26020"); err == nil || !strings.Contains(err.Error(), "device id is invalid") {
		t.Fatalf("Start error = %v", err)
	}
	keptID, err := os.ReadFile(idPath)
	if err != nil || string(keptID) != "nope" {
		t.Fatalf("device id = %q, %v", keptID, err)
	}
	kept, err := os.ReadFile(marker)
	if err != nil || string(kept) != "keep" {
		t.Fatalf("marker = %q, %v", kept, err)
	}
}

func TestStartListensAndShutdownKeepsFiles(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "account-marker.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	seed, err := os.ReadFile(filepath.Join("..", "..", "..", "server", "config", "cn602-save-template.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unused-seed.json"), seed, 0o600); err != nil {
		t.Fatal(err)
	}
	port := freeGamePort(t)
	running, err := Start(dir, strconv.Itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	if running.GamePort != port || running.BattlePort != port+1 {
		t.Fatalf("ports = %d %d", running.GamePort, running.BattlePort)
	}
	dial := func(port int) error {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if err == nil {
			_ = conn.Close()
		}
		return err
	}
	if err := dial(running.GamePort); err != nil {
		t.Fatal(err)
	}
	if err := dial(running.BattlePort); err != nil {
		t.Fatal(err)
	}
	if err := dial(running.GamePort + 2); err == nil {
		t.Fatal("admin port accepted a connection")
	}
	request, err := http.NewRequest(http.MethodPost, running.BaseURL+"/loginSDK.php", strings.NewReader(`{"uuid":"00000000-0000-0000-0000-000000000088","clver":"6.0.7"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "evil.example"
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "该账号不在这台手机上") || strings.Contains(string(body), "api_url") {
		t.Fatalf("login = %d %s", response.StatusCode, body)
	}
	if _, err := Start(dir, strconv.Itoa(port)); err == nil {
		t.Fatal("second start succeeded")
	}
	database, err := sql.Open("sqlite", filepath.Join(dir, "save-state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var players int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cn_local_account WHERE user_id < 1900000000`).Scan(&players); err != nil {
		t.Fatal(err)
	}
	if players != 0 {
		t.Fatalf("player rows = %d", players)
	}
	if err := running.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := dial(running.GamePort); err == nil {
		t.Fatal("game port still open")
	}
	if err := dial(running.BattlePort); err == nil {
		t.Fatal("battle port still open")
	}
	kept, err := os.ReadFile(marker)
	if err != nil || string(kept) != "keep" {
		t.Fatalf("marker = %q, %v", kept, err)
	}
}

func freeGamePort(t *testing.T) int {
	t.Helper()
	for port := 26100; port < 27000; port++ {
		game, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		battle, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port+1))
		if err != nil {
			_ = game.Close()
			continue
		}
		admin, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port+2))
		_ = game.Close()
		_ = battle.Close()
		if err != nil {
			continue
		}
		_ = admin.Close()
		return port
	}
	t.Fatal("no free port")
	return 0
}
