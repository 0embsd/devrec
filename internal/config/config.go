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

// Config holds all devrec configuration.
type Config struct {
	// BaseDir is the root directory for sessions and archives.
	BaseDir string

	// Shell is the shell used for terminal recording. Default: $SHELL or "bash".
	Shell string

	// DefaultCollectors is the list of collectors enabled by default.
	DefaultCollectors []string

	// KeepArchives is the number of recent archives to keep during cleanup.
	KeepArchives int

	// CollectorTimeout is the per-collector execution timeout.
	CollectorTimeout time.Duration

	// PIDDir is where the session PID file is stored.
	PIDDir string
}

// Defaults returns the zero-config default Config.
func Defaults() *Config {
	return &Config{
		BaseDir:            "/opt/devrec",
		Shell:              resolveShell(),
		DefaultCollectors:  []string{"systemd", "ports", "network", "resources", "firewall", "kernel"},
		KeepArchives:       20,
		CollectorTimeout:   15 * time.Second,
		PIDDir:             "/var/run/devrec",
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

func findConfigFile() string {
	// ~/.devrec.yaml
	if u, err := user.Current(); err == nil {
		p := filepath.Join(u.HomeDir, ".devrec.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// /etc/devrec.yaml
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
			fmt.Sscanf(val, "%d", &c.KeepArchives)
		case "collector_timeout":
			d, err := time.ParseDuration(val)
			if err == nil {
				c.CollectorTimeout = d
			}
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
