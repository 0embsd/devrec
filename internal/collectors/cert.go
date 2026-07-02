package collectors

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CertCollector checks TLS certificate expiry.
type CertCollector struct {
	Paths []string // cert files to check, default from config
}

func (c *CertCollector) Name() string { return "cert" }

// CertInfo holds certificate information.
type CertInfo struct {
	Path          string `json:"path"`
	Subject       string `json:"subject,omitempty"`
	Issuer        string `json:"issuer,omitempty"`
	NotBefore     string `json:"not_before,omitempty"`
	NotAfter      string `json:"not_after,omitempty"`
	DaysRemaining int    `json:"days_remaining"`
	Error         string `json:"error,omitempty"`
}

func (c *CertCollector) Collect(ctx context.Context) (interface{}, error) {
	t0 := time.Now()
	paths := c.Paths
	if len(paths) == 0 {
		paths = defaultCertPaths()
	}

	if len(paths) == 0 {
		return make(map[string]*CertInfo), nil
	}

	results := make(map[string]*CertInfo, len(paths))
	for _, path := range paths {
		info := &CertInfo{Path: path}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			info.Error = "not-found"
			results[path] = info
			continue
		}

		out, err := exec.CommandContext(ctx, "openssl", "x509", "-in", path, "-noout", "-dates", "-subject", "-issuer").CombinedOutput()
		if err != nil {
			info.Error = "openssl failed: " + strings.TrimSpace(string(out))
			results[path] = info
			continue
		}

		parseCertOutput(string(out), info)
		results[path] = info
	}
	_ = t0
	return results, nil
}

// defaultCertPaths returns real certificate file paths on the system.
// Only returns actual .pem files, never directories.
func defaultCertPaths() []string {
	var paths []string

	for _, p := range []string{
		"/etc/myx/cert/fullchain.pem",
		"/usr/local/etc/xray/fullchain.pem",
	} {
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}

	if entries, err := os.ReadDir("/etc/letsencrypt/live"); err == nil {
		for _, e := range entries {
			certPath := "/etc/letsencrypt/live/" + e.Name() + "/fullchain.pem"
			if _, err := os.Stat(certPath); err == nil {
				paths = append(paths, certPath)
			}
		}
	}

	return paths
}

func parseCertOutput(output string, info *CertInfo) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "subject=") {
			info.Subject = strings.TrimPrefix(line, "subject=")
		} else if strings.HasPrefix(line, "issuer=") {
			info.Issuer = strings.TrimPrefix(line, "issuer=")
		} else if strings.HasPrefix(line, "notBefore=") {
			info.NotBefore = strings.TrimPrefix(line, "notBefore=")
		} else if strings.HasPrefix(line, "notAfter=") {
			info.NotAfter = strings.TrimPrefix(line, "notAfter=")
			if t, err := time.Parse("Jan  2 15:04:05 2006 MST", info.NotAfter); err == nil {
				info.DaysRemaining = int(time.Until(t).Hours() / 24)
			} else if t, err := time.Parse("Jan 2 15:04:05 2006 MST", info.NotAfter); err == nil {
				info.DaysRemaining = int(time.Until(t).Hours() / 24)
			}
		}
	}
}
