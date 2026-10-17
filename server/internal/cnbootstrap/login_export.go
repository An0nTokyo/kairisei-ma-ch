package cnbootstrap

import (
	"net/http"

	"kairisei.local/server/internal/accountstore"
)

func LoginHandler(accounts *accountstore.Accounts, host string, port int) http.Handler {
	return cnBootstrapLogin(accounts, host, port, CDNConfig{}, resourceVersionNamespace(nil, nil, nil), "images")
}

func ServerListHandler(host string, port int) http.Handler {
	return cnBootstrapServerList(host, port)
}
