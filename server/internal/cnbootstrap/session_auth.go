package cnbootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"kairisei.local/server/internal/accounthttp"
	"kairisei.local/server/internal/accountstore"
)

type cnAuthenticatedUserKey struct{}

func authenticateCNSessions(accounts *accountstore.Accounts) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			request.Header.Del(accounthttp.AccountUserHeader)
			if request.Method != http.MethodPost || isUnauthenticatedCNPost(request.URL.Path) {
				next.ServeHTTP(writer, request)
				return
			}
			body, err := io.ReadAll(io.LimitReader(request.Body, maxCapturedBody+1))
			if err != nil {
				http.Error(writer, "read CN session", http.StatusBadRequest)
				return
			}
			if len(body) > maxCapturedBody {
				http.Error(writer, "CN session payload too large", http.StatusRequestEntityTooLarge)
				return
			}
			request.Body = io.NopCloser(bytes.NewReader(body))
			sessionKey, _, ok := splitCNSessionPayload(body)
			if !ok {
				// Let the concrete route preserve its own protocol error. This also
				// keeps unknown JSON endpoints observable as 501 instead of turning
				// the authentication layer into an endpoint oracle.
				next.ServeHTTP(writer, request)
				return
			}
			userID, err := accounts.ResolveSessionContext(request.Context(), sessionKey)
			if err != nil {
				if errors.Is(err, accountstore.ErrInvalidSession) {
					common := cnBootstrapCommon()
					common["res_code"] = -3208
					common["res_str"] = "登录已失效或服务器已重启，请返回标题重新登录并检查更新。"
					common["res_err_action"] = 1 // Original error dialog: return to title, never delete save data.
					writeCNProtocolResponseWithPopup(writer, common, map[string]any{})
				} else {
					http.Error(writer, "resolve CN account session", http.StatusInternalServerError)
				}
				return
			}
			request = request.WithContext(
				context.WithValue(request.Context(), cnAuthenticatedUserKey{}, userID),
			)
			next.ServeHTTP(writer, request)
		})
	}
}

func refuseAwayLogin(accounts *accountstore.Accounts, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(io.LimitReader(request.Body, maxCapturedBody+1))
		if err != nil {
			http.Error(writer, "read CN local login", http.StatusBadRequest)
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		var payload struct {
			UUID string `json:"uuid"`
		}
		if json.Unmarshal(body, &payload) == nil {
			userID, found, findErr := accounts.FindLogin(payload.UUID)
			if findErr != nil {
				http.Error(writer, "read CN local login", http.StatusBadRequest)
				return
			}
			if found && refuseBorrowedAccount(writer, accounts, userID) {
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

func refuseAwaySessions(accounts *accountstore.Accounts) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			userID, ok := request.Context().Value(cnAuthenticatedUserKey{}).(int)
			if ok && refuseBorrowedAccount(writer, accounts, userID) {
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func refuseBorrowedAccount(writer http.ResponseWriter, accounts *accountstore.Accounts, userID int) bool {
	err := accounts.RefuseIfAwayFromServer(userID)
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "holder") {
		common := cnBootstrapCommon()
		common["res_code"] = -1
		common["res_str"] = "该账号不在这台电脑上"
		common["res_err_action"] = 1
		writeCNProtocolResponseWithPopup(writer, common, map[string]any{})
		return true
	}
	http.Error(writer, "read CN account holder", http.StatusInternalServerError)
	return true
}

func isUnauthenticatedCNPost(path string) bool {
	switch path {
	case "/loginSDK.php", "/mods_switch.php", "/Ping", "/log.php", "/subcribe_push.php":
		return true
	}
	if strings.HasPrefix(path, "/local/sync/") {
		return true
	}
	return strings.HasPrefix(path, "/disabled/envsdk/") || strings.HasPrefix(path, "/d/")
}
