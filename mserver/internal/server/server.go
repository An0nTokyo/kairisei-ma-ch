package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"kairisei.local/mserver/internal/addresses"
	"kairisei.local/mserver/internal/degrade"
	"kairisei.local/server/localcn"
)

type Deps struct {
	Accounts   *localcn.Store
	DeviceID   string
	GamePort   int
	Interfaces func() ([]addresses.Interface, error)
}

type localAddrKey struct{}

func WithLocalAddr(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, localAddrKey{}, host)
}

func localHost(request *http.Request) (string, bool) {
	host, _ := request.Context().Value(localAddrKey{}).(string)
	return host, host != ""
}

func NewHandler(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /loginSDK.php", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveLogin(deps, writer, request)
	}))
	mux.Handle("GET /local/launcher/addresses", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveAddresses(deps, writer, request)
	}))
	list := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveServerList(deps, writer, request)
	})
	mux.Handle("GET /local/server/default.list", list)
	mux.Handle("GET /local/server/apple-review.list", list)
	mux.Handle("GET /local/server/qa.list", list)
	mux.Handle("/", degrade.Handler())
	return mux
}

func serveLogin(deps Deps, writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		http.Error(writer, "read login", http.StatusBadRequest)
		return
	}
	var payload struct {
		UUID string `json:"uuid"`
	}
	_ = json.Unmarshal(body, &payload)
	userID, found, err := deps.Accounts.FindLogin(payload.UUID)
	if err != nil {
		http.Error(writer, "read login", http.StatusBadRequest)
		return
	}
	if !found || deps.Accounts.RefuseIfNotHolder(userID, deps.DeviceID) != nil {
		writePhoneRefusal(writer)
		return
	}
	host, ok := localHost(request)
	if !ok {
		http.Error(writer, "connection address is missing", http.StatusInternalServerError)
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	deps.Accounts.WriteHeldLogin(writer, request, host, deps.GamePort)
}

func writePhoneRefusal(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"res_code": -1,
		"res_str":  "该账号不在这台手机上",
	})
}

func serveServerList(deps Deps, writer http.ResponseWriter, request *http.Request) {
	host, ok := localHost(request)
	if !ok {
		http.Error(writer, "connection address is missing", http.StatusInternalServerError)
		return
	}
	localcn.WriteServerList(writer, request, host, deps.GamePort)
}

func serveAddresses(deps Deps, writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	if deps.Interfaces == nil {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("[]\n"))
		return
	}
	ifaces, err := deps.Interfaces()
	if err != nil {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("[]\n"))
		return
	}
	writer.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(writer).Encode(addresses.Select(ifaces))
}
