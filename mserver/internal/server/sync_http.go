package server

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"kairisei.local/mserver/internal/sync"
	"kairisei.local/server/accountsync"
)

func serveAccountSync(deps Deps, writer http.ResponseWriter, request *http.Request) {
	if !loopbackClient(request.RemoteAddr) {
		http.NotFound(writer, request)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		writeSyncError(writer, http.StatusBadRequest, "account sync body is invalid")
		return
	}
	var payload struct {
		Host   string `json:"host"`
		Port   int    `json:"port"`
		Token  string `json:"token"`
		UserID int    `json:"user_id"`
		Choice string `json:"choice"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || !syncHostOK(payload.Host) || payload.Port < 1 || payload.Port > 65535 || payload.UserID < 1000001 || payload.Token == "" {
		writeSyncError(writer, http.StatusBadRequest, "account sync body is invalid")
		return
	}
	baseURL := "http://" + net.JoinHostPort(payload.Host, strconv.Itoa(payload.Port))
	if request.URL.Path == "/local/launcher/borrow" {
		err = sync.Borrow(baseURL, payload.Token, deps.DeviceID, payload.UserID, deps.DataDir, deps.Accounts)
	} else {
		err = sync.Return(baseURL, payload.Token, deps.DeviceID, payload.UserID, payload.Choice, deps.DataDir, deps.Accounts)
	}
	var choice *accountsync.ChoiceRequiredError
	if errors.As(err, &choice) {
		writeSyncJSON(writer, http.StatusConflict, map[string]any{
			"error":           "choice required",
			"server_revision": choice.ServerRevision,
			"phone_revision":  choice.PhoneRevision,
		})
		return
	}
	if err != nil {
		writeSyncError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeSyncJSON(writer, http.StatusOK, map[string]bool{"ok": true})
}

func syncHostOK(host string) bool {
	if host == "" || strings.ContainsAny(host, " /?#") {
		return false
	}
	return true
}

func loopbackClient(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeSyncJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}

func writeSyncError(writer http.ResponseWriter, status int, message string) {
	writeSyncJSON(writer, status, map[string]string{"error": message})
}
