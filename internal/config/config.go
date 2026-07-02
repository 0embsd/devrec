package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// CollectorSettings holds per-collector configuration.
type CollectorSettings struct {
	SystemdUnits   []string `json:"systemd_units,omitempty"`
	CertPaths      []string `json:"cert_paths,omitempty"`
	FirewallTables []string `json:"firewall_tables,omitempty"`
	PortFlags      string   `json:"port_flags,omitempty"`
}

// Config holds all devrec configuration.
type Config struct {
	BaseDir            string            `json:"dir"`
	Shell              string            `json:"shell,omitempty"`
	DefaultCollectors  []string          `json:"default_collectors"`
	KeepArchives       int               `json:"keep_archives"`
	CollectorTimeout   time.Duration     `json:"collector_timeout"`
	CollectorSettings  CollectorSettings `json:"collector_settings,omitempty"`
	PIDDir             string            `json:"pid_dir,omitempty"`
}

// Defaults returns the zero-config default Config.
func Defaults() *Config {
	return &Config{
		BaseDir:            "/opt/devrec",
		Shell:              resolveShell(),
		DefaultCollectors:  []string{"systemd", "ports", "network", "resources", "firewall", "kernel"},
		KeepArchives:       20,
		CollectorTimeout:   15 * time.Second,
		CollectorSettings: CollectorSettings{
			SystemdUnits:   []string{"xray", "nginx", "ssh", "ufw"},
			CertPaths:      defaultCertPaths(),
			FirewallTables: []string{"filter", "nat"},
		},
		PIDDir: "/var/run/devrec",
	}
}

// Load resolves config from: CLI flags > env vars > config file > defaults.
func Load(flags *pflag.FlagSet) (*Config, error) {
	c := Defaults()

	// 1. Config file (lowest priority).
	configPath := os.Getenv("DEVREC_CONFIG")
	if v, _ := flags.GetString("config"); v != "" {
		configPath = v
	}
	if configPath == "" {
		configPath = findConfigFile()
	}
	if configPath != "" {
		if err := loadYAML(configPath, c); err != nil {
			return nil, fmt.Errorf("config %s: %w", configPath, err)
		}
	}

	// 2. Environment variables.
	if v := os.Getenv("DEVREC_DIR"); v != "" {
		c.BaseDir = v
	}
	if v := os.Getenv("DEVREC_SHELL"); v != "" {
		c.Shell = v
	}
	if v := os.Getenv("DEVREC_COLLECTORS"); v != "" {
		c.DefaultCollectors = parseList(v)
	}
	if v := os.Getenv("DEVREC_KEEP"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			c.KeepArchives = n
		}
	}
	if v := os.Getenv("DEVREC_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.CollectorTimeout = d
		}
	}

	// 3. CLI flags (highest priority).
	if v, _ := flags.GetString("dir"); v != "" {
		c.BaseDir = v
	}

	return c, nil
}

func resolveShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "bash"
}

func defaultCertPaths() []string {
	return []string{
		"/etc/ssl/certs",
		"/etc/nginx/ssl",
		"/etc/xray",
	}
}

func findConfigFile() string {
	if u, err := user.Current(); err == nil {
		p := filepath.Join(u.HomeDir, ".devrec.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if _, err := os.Stat("/etc/devrec.yaml"); err == nil {
		return "/etc/devrec.yaml"
	}
	return ""
}

func parseList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadYAML parses a flat YAML config file (no nesting, key: value only).
func loadYAML(path string, c *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, "\"'")
		switch key {
		case "dir":
			c.BaseDir = val
		case "shell":
			c.Shell = val
		case "default_collectors":
			c.DefaultCollectors = parseList(val)
		case "keep_archives":
			if _, err := fmt.Sscanf(val, "%d", &c.KeepArchives); err != nil {
			fmt.Fprintf(os.Stderr, "devrec: invalid keep_archives value %q\n", val)
		}
		case "collector_timeout":
			d, err := time.ParseDuration(val)
			if err == nil {
				c.CollectorTimeout = d
			}
		case "systemd_units":
			c.CollectorSettings.SystemdUnits = parseList(val)
		case "cert_paths":
			c.CollectorSettings.CertPaths = parseList(val)
		}
	}
	return nil
}

// TempDir returns the active session temp directory path.
func (c *Config) TempDir() string {
	return filepath.Join(c.BaseDir, "tmp")
}

// SessionsDir returns the completed session archives directory path.
func (c *Config) SessionsDir() string {
	return filepath.Join(c.BaseDir, "sessions")
}

// PIDFile returns the path to the session PID file.
func (c *Config) PIDFile() string {
	return filepath.Join(c.PIDDir, "session.pid")
}
