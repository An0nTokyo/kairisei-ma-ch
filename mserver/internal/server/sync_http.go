package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"kairisei.local/mserver/internal/sync"
	"kairisei.local/server/accountsync"
)

var writeSyncLog = func(line string) {
	fmt.Println(line)
}

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
	if err := json.Unmarshal(body, &payload); err != nil {
		writeSyncError(writer, http.StatusBadRequest, "account sync body is invalid")
		return
	}
	if request.URL.Path == "/local/launcher/accounts" {
		if !syncHostOK(payload.Host) || payload.Port < 1 || payload.Port > 65535 || payload.Token == "" {
			writeSyncError(writer, http.StatusBadRequest, "account sync body is invalid")
			return
		}
		baseURL := "http://" + net.JoinHostPort(payload.Host, strconv.Itoa(payload.Port))
		accounts, err := sync.List(baseURL, payload.Token, deps.DeviceID)
		if err != nil {
			logList(writeSyncLog, 0, err)
			writeSyncError(writer, http.StatusBadRequest, err.Error())
			return
		}
		logList(writeSyncLog, len(accounts), nil)
		writeSyncJSON(writer, http.StatusOK, map[string]any{"accounts": accounts})
		return
	}
	if !syncHostOK(payload.Host) || payload.Port < 1 || payload.Port > 65535 || payload.UserID < 1000001 || payload.Token == "" {
		writeSyncError(writer, http.StatusBadRequest, "account sync body is invalid")
		return
	}
	baseURL := "http://" + net.JoinHostPort(payload.Host, strconv.Itoa(payload.Port))
	action := "交还"
	if request.URL.Path == "/local/launcher/borrow" {
		action = "借出"
		err = sync.Borrow(baseURL, payload.Token, deps.DeviceID, payload.UserID, deps.DataDir, deps.Accounts)
	} else if payload.Choice == "phone" {
		action = "保留手机进度"
		err = sync.Return(baseURL, payload.Token, deps.DeviceID, payload.UserID, payload.Choice, deps.DataDir, deps.Accounts)
	} else if payload.Choice == "server" {
		action = "保留电脑进度"
		err = sync.Return(baseURL, payload.Token, deps.DeviceID, payload.UserID, payload.Choice, deps.DataDir, deps.Accounts)
	} else {
		err = sync.Return(baseURL, payload.Token, deps.DeviceID, payload.UserID, payload.Choice, deps.DataDir, deps.Accounts)
	}
	var choice *accountsync.ChoiceRequiredError
	if errors.As(err, &choice) {
		logTransfer(writeSyncLog, action, payload.UserID, nil, choice)
		writeSyncJSON(writer, http.StatusConflict, map[string]any{
			"error":           "choice required",
			"server_revision": choice.ServerRevision,
			"phone_revision":  choice.PhoneRevision,
		})
		return
	}
	logTransfer(writeSyncLog, action, payload.UserID, err, nil)
	if err != nil {
		writeSyncError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if deps.ResetPlay != nil {
		deps.ResetPlay()
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
