package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"

	"kairisei.local/mserver/internal/addresses"
	"kairisei.local/mserver/internal/deviceid"
	"kairisei.local/mserver/internal/port"
	"kairisei.local/server/localcn"
)

type Running struct {
	GamePort   int
	BattlePort int
	DeviceID   string
	BaseURL    string

	store      *localcn.Store
	game       net.Listener
	battle     net.Listener
	httpServer *http.Server
}

func Start(dataDir string, portText string) (*Running, error) {
	gamePort, err := port.Parse(portText)
	if err != nil {
		return nil, err
	}
	deviceID, err := deviceid.LoadOrCreate(dataDir)
	if err != nil {
		return nil, err
	}
	store, err := localcn.Open(
		filepath.Join(dataDir, "save.json"),
		filepath.Join(dataDir, "unused-seed.json"),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err != nil {
		return nil, err
	}
	game, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(gamePort)))
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	battle, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(gamePort+1)))
	if err != nil {
		_ = game.Close()
		_ = store.Close()
		return nil, err
	}
	running := &Running{
		GamePort:   gamePort,
		BattlePort: gamePort + 1,
		DeviceID:   deviceID,
		BaseURL:    "http://127.0.0.1:" + strconv.Itoa(gamePort),
		store:      store,
		game:       game,
		battle:     battle,
	}
	gate := &playGate{
		store:      store,
		dataDir:    dataDir,
		deviceID:   deviceID,
		baseURL:    running.BaseURL,
		battlePort: running.BattlePort,
	}
	running.httpServer = &http.Server{
		Handler: NewHandler(Deps{
			Accounts:   store,
			DeviceID:   deviceID,
			DataDir:    dataDir,
			GamePort:   gamePort,
			Interfaces: currentInterfaces,
			PlayNow:    gate.current,
			ResetPlay:  gate.reset,
		}),
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			tcp, ok := conn.LocalAddr().(*net.TCPAddr)
			if !ok {
				return ctx
			}
			return WithLocalAddr(ctx, tcp.IP.String())
		},
	}
	go func() { _ = running.httpServer.Serve(game) }()
	return running, nil
}

func (running *Running) Shutdown(ctx context.Context) error {
	err := running.httpServer.Shutdown(ctx)
	battleErr := running.battle.Close()
	storeErr := running.store.Close()
	if err != nil {
		return err
	}
	if battleErr != nil && !errors.Is(battleErr, net.ErrClosed) {
		return battleErr
	}
	return storeErr
}

type playGate struct {
	mu         sync.Mutex
	play       *localcn.Play
	failed     bool
	store      *localcn.Store
	dataDir    string
	deviceID   string
	baseURL    string
	battlePort int
}

func (gate *playGate) current() *localcn.Play {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.play != nil || gate.failed {
		return gate.play
	}
	play, err := localcn.OpenPlayFromDir(gate.store, gate.dataDir, gate.deviceID, gate.baseURL, "127.0.0.1", gate.battlePort)
	if err != nil {
		gate.failed = true
		return nil
	}
	gate.play = play
	return play
}

func (gate *playGate) reset() {
	gate.mu.Lock()
	gate.play = nil
	gate.failed = false
	gate.mu.Unlock()
}

func currentInterfaces() ([]addresses.Interface, error) {
	list, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := make([]addresses.Interface, 0, len(list))
	for _, iface := range list {
		item := addresses.Interface{Name: iface.Name, Up: iface.Flags&net.FlagUp != 0}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch value := addr.(type) {
			case *net.IPNet:
				item.Addrs = append(item.Addrs, value.IP)
			case *net.IPAddr:
				item.Addrs = append(item.Addrs, value.IP)
			}
		}
		result = append(result, item)
	}
	return result, nil
}
