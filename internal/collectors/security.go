package collectors

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// SecurityCollector checks SELinux and AppArmor status.
type SecurityCollector struct{}

func (s *SecurityCollector) Name() string { return "security" }

// SecurityInfo holds security module status.
type SecurityInfo struct {
	SELinux  string `json:"selinux"`
	AppArmor string `json:"apparmor"`
}

func (s *SecurityCollector) Collect(ctx context.Context) (interface{}, error) {
	t0 := time.Now()
	info := SecurityInfo{
		SELinux:  "not-installed",
		AppArmor: "not-installed",
	}

	// SELinux: getenforce
	out, err := exec.CommandContext(ctx, "getenforce").Output()
	if err == nil {
		info.SELinux = strings.TrimSpace(string(out))
	}

	// AppArmor: aa-status --enabled
	out, err = exec.CommandContext(ctx, "aa-status", "--enabled").Output()
	if err == nil {
		info.AppArmor = "enabled"
	} else {
		info.AppArmor = "disabled"
	}

	_ = t0
	return info, nil
}
