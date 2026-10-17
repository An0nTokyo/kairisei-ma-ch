package accountsync

import (
	"errors"
	"os"
	"path/filepath"
)

func ReadMasterFiles(dir string) (cards, growth []byte, ok bool, err error) {
	cardsPath := filepath.Join(dir, "masters", "cards.bin")
	growthPath := filepath.Join(dir, "masters", "growth.bin")
	_, cardsErr := os.Stat(cardsPath)
	_, growthErr := os.Stat(growthPath)
	if errors.Is(cardsErr, os.ErrNotExist) && errors.Is(growthErr, os.ErrNotExist) {
		return nil, nil, false, nil
	}
	cards, err = os.ReadFile(cardsPath)
	if err != nil {
		return nil, nil, false, err
	}
	growth, err = os.ReadFile(growthPath)
	if err != nil {
		return nil, nil, false, err
	}
	return cards, growth, true, nil
}

func WriteMasterFiles(dir string, cards, growth []byte) error {
	masterDir := filepath.Join(dir, "masters")
	if err := os.MkdirAll(masterDir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(masterDir, "cards.bin"), cards, 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(masterDir, "growth.bin"), growth, 0o600)
}

func DefenseFilePath(dir string) string {
	return filepath.Join(dir, "defense-decks.json")
}

func WriteDefenseDecks(dir string, body []byte) error {
	if len(body) == 0 {
		return nil
	}
	return os.WriteFile(DefenseFilePath(dir), body, 0o600)
}

func WriteExtraMasterFiles(dir string, files map[string][]byte) error {
	if len(files) == 0 {
		return nil
	}
	masterDir := filepath.Join(dir, "masters")
	if err := os.MkdirAll(masterDir, 0o700); err != nil {
		return err
	}
	for name, body := range files {
		if _, ok := extraMasterNames[name]; !ok {
			return errors.New("account master name is invalid")
		}
		if err := os.WriteFile(filepath.Join(masterDir, name), body, 0o600); err != nil {
			return err
		}
	}
	return nil
}

var extraMasterNames = map[string]struct{}{
	"explore.bin":     {},
	"story.bin":       {},
	"navi.bin":        {},
	"items.bin":       {},
	"avatar.bin":      {},
	"stamps.bin":      {},
	"honors.bin":      {},
	"login-bonus.bin": {},
	"battle.bin":      {},
	"pvp.bin":         {},
}
