package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kairisei.local/mserver/internal/addresses"
	"kairisei.local/mserver/internal/degrade"
	"kairisei.local/mserver/internal/solo"
	"kairisei.local/server/localcn"
)

type Deps struct {
	Accounts   *localcn.Store
	DeviceID   string
	DataDir    string
	GamePort   int
	Interfaces func() ([]addresses.Interface, error)
	Play       *localcn.Play
	PlayNow    func() *localcn.Play
	ResetPlay  func()
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
	catalog, cpkList, cpkState := readResourceCatalogs(deps.DataDir)
	mux := http.NewServeMux()
	mux.Handle("POST /loginSDK.php", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveLogin(deps, catalog, cpkList, cpkState, writer, request)
	}))
	mux.Handle("POST /mods_switch.php", http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		localcn.WriteModuleSwitches(writer)
	}))
	mux.Handle("GET /disabled/products", http.HandlerFunc(localcn.WriteProducts))
	intro := http.HandlerFunc(localcn.WritePlaceholderPNG)
	mux.Handle("GET /disabled/web/netease/fourplusone/20160626/Intro_0.png", intro)
	mux.Handle("GET /disabled/web/netease/fourplusone/20160626/Intro_1.png", intro)
	mux.Handle("GET /disabled/web/netease/fourplusone/20160626/Intro_2.png", intro)
	mux.Handle("GET /disabled/web/netease/fourplusone/20160626/Intro_3.png", intro)
	mux.Handle("GET /disabled/web/netease/fourplusone/20160626/Intro_4.png", intro)
	mux.Handle("GET /local/version/{namespace}/Android/patch/catalog.dat", writeStored("application/octet-stream", catalog))
	mux.Handle("GET /local/version/{namespace}/CPK/cpk_file.csv", writeStored("text/plain; charset=utf-8", cpkList))
	mux.Handle("GET /local/version/{namespace}/CPK/cpk_patch.txt", writeStored("text/plain; charset=utf-8", cpkState))
	banners := serveLocalBanners(bannerRoot(deps.DataDir))
	mux.Handle("GET /local/home/{path...}", banners)
	mux.Handle("HEAD /local/home/{path...}", banners)
	mux.Handle("GET /local/gacha/{path...}", banners)
	mux.Handle("HEAD /local/gacha/{path...}", banners)
	downloads := proxyResourceDownloads(readResourceUpstream(deps.DataDir))
	mux.Handle("GET /local/resources/patch/{path...}", downloads)
	mux.Handle("HEAD /local/resources/patch/{path...}", downloads)
	mux.Handle("GET /local/resources/cpk/{path...}", downloads)
	mux.Handle("HEAD /local/resources/cpk/{path...}", downloads)
	if deps.Accounts != nil {
		inner := deps.Accounts.AccountHandler()
		account := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			capture := &routeCapture{ResponseWriter: writer}
			inner.ServeHTTP(capture, request)
			// #region agent log
			routeLog(deps.DataDir, "account", request.URL.Path, capture.summary())
			// #endregion
		})
		mux.Handle("POST /local/account/status", account)
		mux.Handle("POST /local/account/bind", account)
		mux.Handle("POST /local/account/login", account)
	}
	mux.Handle("GET /local/launcher/addresses", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveAddresses(deps, writer, request)
	}))
	list := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveServerList(deps, writer, request)
	})
	mux.Handle("GET /local/server/default.list", list)
	mux.Handle("GET /local/server/apple-review.list", list)
	mux.Handle("GET /local/server/qa.list", list)
	accountSync := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveAccountSync(deps, writer, request)
	})
	mux.Handle("POST /local/launcher/borrow", accountSync)
	mux.Handle("POST /local/launcher/return", accountSync)
	mux.Handle("POST /local/launcher/accounts", accountSync)
	mux.Handle("POST /local/launcher/banners", accountSync)
	mux.Handle("/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		play := deps.currentPlay()
		if solo.Dispatch(play, writer, request) {
			return
		}
		// #region agent log
		routeLog(deps.DataDir, "degrade", request.Method+" "+request.URL.Path, play == nil)
		// #endregion
		degrade.Handler().ServeHTTP(writer, request)
	}))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "//") {
			request.URL.Path = "/" + strings.TrimLeft(request.URL.Path, "/")
		}
		capture := &routeCapture{ResponseWriter: writer}
		mux.ServeHTTP(capture, request)
		// #region agent log
		routeLog(deps.DataDir, "call", request.Method+" "+request.URL.Path, capture.summary())
		routeLog(deps.DataDir, "size", request.Method+" "+request.URL.Path, capture.written)
		// #endregion
	})
}

// #region agent log
type routeCapture struct {
	http.ResponseWriter
	status  int
	body    bytes.Buffer
	written int
}

func (capture *routeCapture) WriteHeader(status int) {
	capture.status = status
	capture.ResponseWriter.WriteHeader(status)
}

func (capture *routeCapture) Write(data []byte) (int, error) {
	if capture.status == 0 {
		capture.status = http.StatusOK
	}
	capture.written += len(data)
	if capture.body.Len() < 4096 {
		remain := 4096 - capture.body.Len()
		if len(data) > remain {
			capture.body.Write(data[:remain])
		} else {
			capture.body.Write(data)
		}
	}
	return capture.ResponseWriter.Write(data)
}

func (capture *routeCapture) summary() map[string]any {
	summary := map[string]any{"status": capture.status, "bytes": capture.body.Len()}
	var payload map[string]any
	if json.NewDecoder(bytes.NewReader(capture.body.Bytes())).Decode(&payload) == nil {
		summary["res_code"] = payload["res_code"]
		summary["res_str"] = payload["res_str"]
		summary["res_err_action"] = payload["res_err_action"]
		summary["state"] = payload["state"]
		summary["error"] = payload["error"]
	}
	return summary
}

func routeLog(dataDir, kind, message string, detail any) {
	line, err := json.Marshal(map[string]any{
		"sessionId": "9a1738", "runId": "feature", "hypothesisId": "D",
		"location": "server.go:route", "message": message, "timestamp": time.Now().UnixMilli(),
		"data": map[string]any{"kind": kind, "detail": detail},
	})
	if err != nil {
		return
	}
	file, err := os.OpenFile(filepath.Join(dataDir, "debug-9a1738.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = file.Write(append(line, '\n'))
	_ = file.Close()
}

// #endregion

func (deps Deps) currentPlay() *localcn.Play {
	if deps.PlayNow != nil {
		return deps.PlayNow()
	}
	return deps.Play
}

func serveLogin(deps Deps, catalog, cpkList, cpkState []byte, writer http.ResponseWriter, request *http.Request) {
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
	deps.Accounts.WriteHeldLogin(writer, request, host, deps.GamePort, catalog, cpkList, cpkState)
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

func readResourceUpstream(dir string) string {
	if dir == "" {
		return ""
	}
	body, err := os.ReadFile(filepath.Join(dir, "resource-upstream"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

func proxyResourceDownloads(upstream string) http.Handler {
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if upstream == "" {
			degrade.Handler().ServeHTTP(writer, request)
			return
		}
		target, err := url.Parse(upstream)
		if err != nil || target.Scheme == "" || target.Host == "" {
			degrade.Handler().ServeHTTP(writer, request)
			return
		}
		target.Path = request.URL.Path
		target.RawQuery = request.URL.RawQuery
		upstreamRequest, err := http.NewRequestWithContext(request.Context(), request.Method, target.String(), nil)
		if err != nil {
			degrade.Handler().ServeHTTP(writer, request)
			return
		}
		if header := request.Header.Get("Range"); header != "" {
			upstreamRequest.Header.Set("Range", header)
		}
		response, err := client.Do(upstreamRequest)
		if err != nil {
			degrade.Handler().ServeHTTP(writer, request)
			return
		}
		defer response.Body.Close()
		for key, values := range response.Header {
			for _, value := range values {
				writer.Header().Add(key, value)
			}
		}
		writer.WriteHeader(response.StatusCode)
		if request.Method != http.MethodHead {
			_, _ = io.Copy(writer, response.Body)
		}
	})
}

func readResourceCatalogs(dir string) (catalog, cpkList, cpkState []byte) {
	if dir == "" {
		return nil, nil, nil
	}
	base := filepath.Join(dir, "resources")
	catalog, catalogErr := os.ReadFile(filepath.Join(base, "catalog.dat"))
	cpkList, listErr := os.ReadFile(filepath.Join(base, "cpk_file.csv"))
	cpkState, stateErr := os.ReadFile(filepath.Join(base, "cpk_patch.txt"))
	if catalogErr != nil || listErr != nil || stateErr != nil {
		return nil, nil, nil
	}
	return catalog, cpkList, cpkState
}

func writeStored(contentType string, body []byte) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", contentType)
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(http.StatusOK)
		if len(body) > 0 {
			_, _ = writer.Write(body)
		}
	}
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
