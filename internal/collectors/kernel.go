package collectors

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// KernelCollector captures kernel and OS info.
type KernelCollector struct{}

func (k *KernelCollector) Name() string { return "kernel" }

// KernelInfo holds kernel/OS information.
type KernelInfo struct {
	OSRelease map[string]string `json:"os_release"`
	Uname     string            `json:"uname"`
}

func (k *KernelCollector) Collect(ctx context.Context) (interface{}, error) {
	t0 := time.Now()
	info := KernelInfo{
		OSRelease: make(map[string]string),
	}

	// /etc/os-release
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			info.OSRelease[key] = strings.Trim(val, `"'`)
		}
	}

	// uname -a
	if out, err := exec.CommandContext(ctx, "uname", "-a").Output(); err == nil {
		info.Uname = strings.TrimSpace(string(out))
	}

	_ = t0
	return info, nil
}
