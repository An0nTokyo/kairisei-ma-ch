package localcn

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"kairisei.local/server/accountsync"
	"kairisei.local/server/internal/cnbootstrap"
	"kairisei.local/server/internal/multiplayer"
)

type PlayOptions struct {
	DeviceID          string
	BaseURL           string
	Logger            *slog.Logger
	Cards             string
	Explore           string
	Story             string
	Navi              string
	Items             string
	Avatar            string
	Stamps            string
	Honors            string
	PlayerProgression string
	LoginBonus        string
	Battle            string
	PVP               string
	BattleHost        string
	BattlePort        int
	DefenseDecks      []byte
}

type Play struct {
	Handler http.Handler
	paths   map[string]struct{}
}

func OpenPlayFromDir(store *Store, dataDir, deviceID, baseURL, battleHost string, battlePort int) (*Play, error) {
	masters := filepath.Join(dataDir, "masters")
	optional := func(name string) string {
		path := filepath.Join(masters, name)
		if _, err := os.Stat(path); err != nil {
			return ""
		}
		return path
	}
	var defense []byte
	defensePath := accountsync.DefenseFilePath(dataDir)
	if body, err := os.ReadFile(defensePath); err == nil {
		defense = body
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return OpenPlay(store, PlayOptions{
		DeviceID:          deviceID,
		BaseURL:           baseURL,
		Cards:             filepath.Join(masters, "cards.bin"),
		Explore:           filepath.Join(masters, "explore.bin"),
		Story:             filepath.Join(masters, "story.bin"),
		Navi:              filepath.Join(masters, "navi.bin"),
		Items:             filepath.Join(masters, "items.bin"),
		Avatar:            filepath.Join(masters, "avatar.bin"),
		Stamps:            filepath.Join(masters, "stamps.bin"),
		Honors:            filepath.Join(masters, "honors.bin"),
		PlayerProgression: filepath.Join(masters, "growth.bin"),
		LoginBonus:        filepath.Join(masters, "login-bonus.bin"),
		Battle:            optional("battle.bin"),
		PVP:               optional("pvp.bin"),
		BattleHost:        battleHost,
		BattlePort:        battlePort,
		DefenseDecks:      defense,
	})
}

func OpenPlay(store *Store, options PlayOptions) (*Play, error) {
	handler, paths, err := cnbootstrap.NewSoloPlay(store.accounts, cnbootstrap.SoloPlayConfig{
		DeviceID: options.DeviceID,
		BaseURL:  options.BaseURL,
		Logger:   options.Logger,
		Masters: cnbootstrap.MastersConfig{
			Cards:             options.Cards,
			Explore:           options.Explore,
			Story:             options.Story,
			Battle:            options.Battle,
			Navi:              options.Navi,
			Items:             options.Items,
			Avatar:            options.Avatar,
			Stamps:            options.Stamps,
			Honors:            options.Honors,
			PVP:               options.PVP,
			PlayerProgression: options.PlayerProgression,
			LoginBonus:        options.LoginBonus,
		},
		BattleSV:     multiplayer.Endpoint{Host: options.BattleHost, Port: uint16(options.BattlePort)},
		DefenseDecks: options.DefenseDecks,
	})
	if err != nil {
		return nil, err
	}
	play := &Play{Handler: handler, paths: make(map[string]struct{}, len(paths))}
	for _, path := range paths {
		play.paths[path] = struct{}{}
	}
	return play, nil
}

func (play *Play) Allows(method, path string) bool {
	if play == nil || method != http.MethodPost {
		return false
	}
	_, ok := play.paths[path]
	return ok
}

func (play *Play) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	play.Handler.ServeHTTP(writer, request)
}
