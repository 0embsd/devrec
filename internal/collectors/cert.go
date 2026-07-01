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

func defaultCertPaths() []string {
	var paths []string
	candidates := []string{
		"/etc/myx/cert/fullchain.pem",
		"/etc/letsencrypt/live",
		"/etc/ssl/certs",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		paths = append(paths, "/etc/ssl/certs")
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
			// Parse date and compute days remaining.
			if t, err := time.Parse("Jan  2 15:04:05 2006 MST", info.NotAfter); err == nil {
				info.DaysRemaining = int(time.Until(t).Hours() / 24)
			} else if t, err := time.Parse("Jan 2 15:04:05 2006 MST", info.NotAfter); err == nil {
				info.DaysRemaining = int(time.Until(t).Hours() / 24)
			}
		}
	}
}

