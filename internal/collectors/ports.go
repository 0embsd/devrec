package collectors

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// PortsCollector captures listening ports via ss.
type PortsCollector struct{}

func (p *PortsCollector) Name() string { return "ports" }

// PortEntry represents a single listening socket.
type PortEntry struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Process  string `json:"process"`
}

func (p *PortsCollector) Collect(ctx context.Context) (interface{}, error) {
	t0 := time.Now()
	// Try ss -tlnp first (needs root for process names).
	out, err := exec.CommandContext(ctx, "ss", "-tlnp").Output()
	if err != nil {
		// Fallback: ss -tln without process info.
		out, err = exec.CommandContext(ctx, "ss", "-tln").Output()
		if err != nil {
			_ = t0
			return nil, err
		}
		return parseSS(string(out), false), nil
	}
	return parseSS(string(out), true), nil
}

func parseSS(output string, hasProc bool) []PortEntry {
	var entries []PortEntry
	lines := strings.Split(output, "\n")
	for _, line := range lines[1:] { // skip header
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		entry := PortEntry{Protocol: "tcp"}
		if strings.HasPrefix(fields[0], "u") {
			entry.Protocol = "udp"
		}

		// Parse LocalAddress:Port (field 4).
		addr := fields[3]
		if idx := strings.LastIndex(addr, ":"); idx >= 0 {
			entry.Address = addr[:idx]
			portStr := addr[idx+1:]
			// Handle IPv6 [::1]:443 format.
			entry.Address = strings.Trim(entry.Address, "[]")
			if p, err := strconv.Atoi(portStr); err == nil {
				entry.Port = p
			}
		}

		if hasProc && len(fields) >= 6 {
			// Process is in the last field(s), may contain spaces.
			entry.Process = strings.Join(fields[5:], " ")
			// Extract just the process name from "users:(("nginx",pid=..." format.
			if idx := strings.Index(entry.Process, `"`); idx >= 0 {
				rest := entry.Process[idx+1:]
				if end := strings.Index(rest, `"`); end >= 0 {
					entry.Process = rest[:end]
				}
			}
		}

		entries = append(entries, entry)
	}
	return entries
}

