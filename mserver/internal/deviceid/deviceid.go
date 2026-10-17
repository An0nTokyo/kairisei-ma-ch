package deviceid

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func LoadOrCreate(dir string) (string, error) {
	path := filepath.Join(dir, "device-id")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		id, err := newID()
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(id), 0o600); err != nil {
			return "", err
		}
		return id, nil
	}
	if err != nil {
		return "", err
	}
	id := strings.ToLower(strings.TrimSpace(string(content)))
	if !valid(id) {
		return "", errors.New("device id is invalid")
	}
	return id, nil
}

func valid(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	const hexdigits = "0123456789abcdef"
	buf := make([]byte, 36)
	position := 0
	for index, value := range raw {
		if index == 4 || index == 6 || index == 8 || index == 10 {
			buf[position] = '-'
			position++
		}
		buf[position] = hexdigits[value>>4]
		buf[position+1] = hexdigits[value&0x0f]
		position += 2
	}
	return string(buf), nil
}
