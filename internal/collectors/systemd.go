package collectors

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// SystemdCollector checks systemd unit states.
type SystemdCollector struct {
	Units []string // configured units to check
}

func (s *SystemdCollector) Name() string { return "systemd" }

func (s *SystemdCollector) Collect(ctx context.Context) (interface{}, error) {
	t0 := time.Now()
	result := make(map[string]string, len(s.Units))
	for _, unit := range s.Units {
		// systemctl is-active is read-only, no root required.
		out, err := exec.CommandContext(ctx, "systemctl", "is-active", unit).Output()
		status := strings.TrimSpace(string(out))
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				switch code {
				case 3:
					status = "inactive"
				case 4:
					status = "not-found"
				default:
					status = "error"
				}
			} else {
				status = "error: " + err.Error()
			}
		}
		result[unit] = status
	}
	_ = t0
	return result, nil
}
