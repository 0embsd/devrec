package collectors

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// NetworkCollector captures network interface info via ip.
type NetworkCollector struct{}

func (n *NetworkCollector) Name() string { return "network" }

// InterfaceEntry represents a network interface.
type InterfaceEntry struct {
	Name      string   `json:"name"`
	State     string   `json:"state"`
	Addresses []string `json:"addresses"`
}

func (n *NetworkCollector) Collect(ctx context.Context) (interface{}, error) {
	t0 := time.Now()
	// Try ip -j (JSON output, available since iproute2 4.13 / 2017).
	out, err := exec.CommandContext(ctx, "ip", "-j", "addr", "show").Output()
	if err == nil {
		var raw []struct {
			Name      string `json:"ifname"`
			OperState string `json:"operstate"`
			AddrInfo  []struct {
				Local string `json:"local"`
			} `json:"addr_info"`
		}
		if err := json.Unmarshal(out, &raw); err == nil {
			var entries []InterfaceEntry
			for _, r := range raw {
				e := InterfaceEntry{Name: r.Name, State: r.OperState}
				for _, a := range r.AddrInfo {
					e.Addresses = append(e.Addresses, a.Local)
				}
				entries = append(entries, e)
			}
			_ = t0
			return entries, nil
		}
	}

	// Fallback: plain text parsing.
	out, err = exec.CommandContext(ctx, "ip", "addr", "show").Output()
	if err != nil {
		return nil, err
	}
	_ = t0
	return parseIPAddr(string(out)), nil
}

func parseIPAddr(output string) []InterfaceEntry {
	var entries []InterfaceEntry
	var cur *InterfaceEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, ": "); idx >= 0 && len(line) > 0 && line[0] >= '0' && line[0] <= '9' {
			if cur != nil {
				entries = append(entries, *cur)
			}
			rest := line[idx+2:]
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				name := strings.TrimSuffix(parts[0], ":")
				cur = &InterfaceEntry{Name: name}
				for _, p := range parts {
					if p == "UP" {
						cur.State = "UP"
					} else if p == "DOWN" {
						cur.State = "DOWN"
					}
				}
			}
		}
		if strings.HasPrefix(line, "inet ") && cur != nil {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				cur.Addresses = append(cur.Addresses, fields[1])
			}
		}
	}
	if cur != nil {
		entries = append(entries, *cur)
	}
	return entries
}
