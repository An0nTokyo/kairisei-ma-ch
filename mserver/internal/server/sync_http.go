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
	if request.URL.Path == "/local/launcher/banners" {
		if !syncHostOK(payload.Host) || payload.Port < 1 || payload.Port > 65535 || payload.Token == "" {
			writeSyncError(writer, http.StatusBadRequest, "account sync body is invalid")
			return
		}
		baseURL := "http://" + net.JoinHostPort(payload.Host, strconv.Itoa(payload.Port))
		files, missing, err := sync.FetchBanners(baseURL, payload.Token)
		if err != nil {
			writeSyncLog("同步图片失败: " + err.Error())
			writeSyncError(writer, http.StatusBadRequest, err.Error())
			return
		}
		converted := make([]bannerSyncFile, 0, len(files))
		for _, file := range files {
			converted = append(converted, bannerSyncFile{Path: file.Path, SHA256: file.SHA256, Bytes: file.Bytes})
		}
		synced, err := writeBannerFiles(bannerRoot(deps.DataDir), converted)
		if err != nil {
			writeSyncLog("同步图片失败: " + err.Error())
			writeSyncError(writer, http.StatusBadRequest, err.Error())
			return
		}
		if missing > 0 {
			writeSyncLog(fmt.Sprintf("同步图片 %d 张，缺 %d 张", synced, missing))
		} else {
			writeSyncLog(fmt.Sprintf("同步图片 %d 张", synced))
		}
		writeSyncJSON(writer, http.StatusOK, map[string]any{"synced": synced, "missing": missing})
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
