package cnbootstrap

import (
	"net/http"

	"kairisei.local/server/internal/accountstore"
)

func LoginHandler(accounts *accountstore.Accounts, host string, port int) http.Handler {
	return LoginHandlerForCatalogs(accounts, host, port, nil, nil, nil)
}

func LoginHandlerForCatalogs(accounts *accountstore.Accounts, host string, port int, catalog, cpkList, cpkState []byte) http.Handler {
	return cnBootstrapLogin(accounts, host, port, CDNConfig{}, resourceVersionNamespace(catalog, cpkList, cpkState), "images")
}

func ResourceVersionNamespace(catalog, cpkList, cpkState []byte) string {
	return resourceVersionNamespace(catalog, cpkList, cpkState)
}

func ServerListHandler(host string, port int) http.Handler {
	return cnBootstrapServerList(host, port)
}
