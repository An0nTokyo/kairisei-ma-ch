package addresses

import (
	"net"
	"sort"
)

type Interface struct {
	Name  string
	Up    bool
	Addrs []net.IP
}

type Row struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Loopback bool   `json:"loopback"`
	Label    string `json:"label"`
}

func Select(interfaces []Interface) []Row {
	linkLocal := &net.IPNet{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)}
	var rows []Row
	for _, iface := range interfaces {
		if !iface.Up {
			continue
		}
		for _, addr := range iface.Addrs {
			ip := addr.To4()
			if ip == nil || ip.Equal(net.IPv4zero) || linkLocal.Contains(ip) {
				continue
			}
			if ip.IsLoopback() {
				if !ip.Equal(net.IPv4(127, 0, 0, 1)) {
					continue
				}
				rows = append(rows, Row{Name: iface.Name, Host: "127.0.0.1", Loopback: true, Label: "本机 · " + iface.Name})
				continue
			}
			rows = append(rows, Row{Name: iface.Name, Host: ip.String(), Loopback: false, Label: iface.Name})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Loopback != rows[j].Loopback {
			return rows[i].Loopback
		}
		return rows[i].Host < rows[j].Host
	})
	if rows == nil {
		return []Row{}
	}
	return rows
}
