package cnbootstrap

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *application) registerPhoneSafeRoutes(router chi.Router) []string {
	var paths []string
	post := func(path string, handler http.HandlerFunc) {
		paths = append(paths, path)
		router.Post(path, handler)
	}
	post("/FollowoFollowShow", phoneSafeFollowShow)
	post("/FollowFollowerShow", phoneSafeFollowerShow)
	post("/FriendSearch", phoneSafeFriendSearch)
	// Tasks 2–4 append more posts here (or register all nine in one file as you implement).
	return paths
}

func phoneSafeFollowShow(writer http.ResponseWriter, request *http.Request) {
	if err := requireCNSessionOnly(request, "FollowoFollowShow"); err != nil {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}
	writeCNProtocolResponseWithPopup(writer, cnBootstrapCommon(), map[string]any{
		"follow_max": 80,
		"friends":    []any{},
	})
}

func phoneSafeFollowerShow(writer http.ResponseWriter, request *http.Request) {
	if _, err := normalizeCNOptionalFields(request, "FollowFollowerShow", []string{"page"}, []string{"page"}); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writeCNProtocolResponseWithPopup(writer, cnBootstrapCommon(), map[string]any{
		"friends":      []any{},
		"follower_num": 0,
		"page_max":     1,
	})
}

func phoneSafeFriendSearch(writer http.ResponseWriter, request *http.Request) {
	if _, err := normalizeCNOptionalFields(request, "FriendSearch", []string{"inviteid"}, []string{"inviteid"}); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	writeCNProtocolResponseWithPopup(writer, cnBootstrapCommon(), map[string]any{
		"user": []any{},
	})
}
