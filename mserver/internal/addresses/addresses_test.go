package addresses

import (
	"net"
	"testing"
)

func TestSelectKeepsUsableIPv4Only(t *testing.T) {
	rows := Select([]Interface{
		{Name: "lo", Up: true, Addrs: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("127.0.0.2"),
		}},
		{Name: "wlan0", Up: true, Addrs: []net.IP{
			net.ParseIP("192.168.1.23"),
			net.ParseIP("0.0.0.0"),
			net.ParseIP("169.254.4.5"),
			net.ParseIP("fe80::1"),
		}},
		{Name: "down0", Up: false, Addrs: []net.IP{net.ParseIP("10.0.0.8")}},
	})
	if len(rows) != 2 {
		t.Fatalf("rows = %#v", rows)
	}
	if rows[0].Label != "本机 · lo" || rows[0].Host != "127.0.0.1" || !rows[0].Loopback || rows[0].Name != "lo" {
		t.Fatalf("loopback row = %#v", rows[0])
	}
	if rows[1].Label != "wlan0" || rows[1].Host != "192.168.1.23" || rows[1].Loopback || rows[1].Name != "wlan0" {
		t.Fatalf("wifi row = %#v", rows[1])
	}
}
