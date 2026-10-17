package solo

import (
	"net/http"

	"kairisei.local/server/localcn"
)

func Dispatch(play *localcn.Play, writer http.ResponseWriter, request *http.Request) bool {
	if play == nil || !play.Allows(request.Method, request.URL.Path) {
		return false
	}
	play.ServeHTTP(writer, request)
	return true
}
