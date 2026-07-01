package collectors

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// CustomCollector runs user-specified shell commands and captures their output.
type CustomCollector struct {
	Label    string
	Commands []string
}

func (c *CustomCollector) Name() string { return c.Label }

func (c *CustomCollector) Collect(ctx context.Context) (interface{}, error) {
	t0 := time.Now()
	results := make(map[string]string, len(c.Commands))
	for _, cmd := range c.Commands {
		out, err := exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
		if err != nil {
			results[cmd] = "ERROR: " + strings.TrimSpace(string(out))
		} else {
			results[cmd] = strings.TrimSpace(string(out))
		}
	}
	_ = t0
	return results, nil
}
