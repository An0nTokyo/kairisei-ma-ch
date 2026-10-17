package deviceid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateDeviceID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "device-id")

	first, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !valid(first) {
		t.Fatalf("created id %q", first)
	}
	created, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if second != first || string(again) != string(created) {
		t.Fatalf("second = %q file = %q, want %q", second, again, created)
	}

	upper := "ABCDEFAB-CDEF-ABCD-EFAB-CDEFABCDEFAB"
	if err := os.WriteFile(path, []byte(upper), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.ToLower(upper) || string(kept) != upper {
		t.Fatalf("got %q file %q", got, kept)
	}

	if err := os.WriteFile(path, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreate(dir); err == nil || !strings.Contains(err.Error(), "device id is invalid") {
		t.Fatalf("error = %v", err)
	}
	left, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(left) != "nope" {
		t.Fatalf("file changed to %q", left)
	}
}
