package port

import (
	"strings"
	"testing"
)

func TestParsePort(t *testing.T) {
	got, err := Parse("26020")
	if err != nil || got != 26020 {
		t.Fatalf("Parse(26020) = %d, %v", got, err)
	}
	for _, text := range []string{"1", "65534"} {
		value, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if text == "1" && value != 1 || text == "65534" && value != 65534 {
			t.Fatalf("Parse(%s) = %d", text, value)
		}
	}
	for _, text := range []string{"", "0", "01", "65535", "26020 ", "+26020"} {
		if _, err := Parse(text); err == nil || !strings.Contains(err.Error(), "本机端口无效") {
			t.Fatalf("Parse(%q) error = %v", text, err)
		}
	}
}
