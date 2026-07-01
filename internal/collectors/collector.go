package collectors

import (
	"context"
	"time"
)

// Result holds a named collector's output.
type Result struct {
	Collector string      `json:"collector"`
	Success   bool        `json:"success"`
	Data      interface{} `json:"data"`
	Error     string      `json:"error,omitempty"`
	Duration  string      `json:"duration"`
}

// Collector inspects system state and returns structured data.
type Collector interface {
	Name() string
	Collect(ctx context.Context) (interface{}, error)
}

// Registry holds all available collectors, keyed by name.
type Registry map[string]Collector

// DefaultRegistry returns the built-in set of collectors.
func DefaultRegistry() Registry {
	return Registry{
		"systemd":   &SystemdCollector{Units: []string{"xray", "nginx", "fail2ban", "ssh", "ssh.socket"}},
		"ports":     &PortsCollector{},
		"network":   &NetworkCollector{},
		"cert":      &CertCollector{},
		"resources": &ResourcesCollector{},
		"firewall":  &FirewallCollector{},
		"kernel":    &KernelCollector{},
		"security":  &SecurityCollector{},
	}
}

// DefaultCollectorNames returns the default ordered list of collector names.
func DefaultCollectorNames() []string {
	return []string{"systemd", "ports", "network", "resources", "firewall", "kernel"}
}

// FilterRegistry returns collectors matching the requested names, in order.
// Names may include "custom:label=cmd" syntax for custom collectors.
func FilterRegistry(reg Registry, names []string) ([]Collector, error) {
	var out []Collector
	for _, name := range names {
		if c, ok := reg[name]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

// okResult wraps a successful collection.
func okResult(name string, data interface{}, d time.Duration) Result {
	return Result{
		Collector: name,
		Success:   true,
		Data:      data,
		Duration:  d.String(),
	}
}

// errResult wraps a failed collection.
func errResult(name string, err error, d time.Duration) Result {
	return Result{
		Collector: name,
		Success:   false,
		Data:      nil,
		Error:     err.Error(),
		Duration:  d.String(),
	}
}
