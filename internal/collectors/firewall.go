package collectors

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// FirewallCollector captures firewall state.
type FirewallCollector struct{}

func (f *FirewallCollector) Name() string { return "firewall" }

// FirewallSnapshot holds firewall information.
type FirewallSnapshot struct {
	UFW      string `json:"ufw"`
	Iptables string `json:"iptables"`
}

func (f *FirewallCollector) Collect(ctx context.Context) (interface{}, error) {
	t0 := time.Now()
	snap := FirewallSnapshot{}

	// ufw status (may need root).
	out, err := exec.CommandContext(ctx, "ufw", "status", "verbose").CombinedOutput()
	if err != nil {
		snap.UFW = "error: " + strings.TrimSpace(string(out))
	} else {
		snap.UFW = strings.TrimSpace(string(out))
	}

	// iptables -L -n (may need root).
	out, err = exec.CommandContext(ctx, "iptables", "-L", "-n").CombinedOutput()
	if err != nil {
		snap.Iptables = "error: " + strings.TrimSpace(string(out))
	} else {
		snap.Iptables = strings.TrimSpace(string(out))
	}

	_ = t0
	return snap, nil
}
