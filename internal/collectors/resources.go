package collectors

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ResourcesCollector captures system resource usage.
type ResourcesCollector struct{}

func (r *ResourcesCollector) Name() string { return "resources" }

// ResourceSnapshot holds resource information.
type ResourceSnapshot struct {
	Disk     []DiskEntry `json:"disk"`
	Memory   MemoryInfo  `json:"memory"`
	Uptime   string      `json:"uptime"`
	LoadAvg  string      `json:"load_avg"`
}

// DiskEntry represents a filesystem entry from df.
type DiskEntry struct {
	Filesystem string `json:"filesystem"`
	Size       string `json:"size"`
	Used       string `json:"used"`
	Available  string `json:"available"`
	UsePercent string `json:"use_percent"`
	MountPoint string `json:"mount_point"`
}

// MemoryInfo holds memory info from free.
type MemoryInfo struct {
	Total     string `json:"total"`
	Used      string `json:"used"`
	Free      string `json:"free"`
	Available string `json:"available"`
}

func (r *ResourcesCollector) Collect(ctx context.Context) (interface{}, error) {
	t0 := time.Now()
	snap := ResourceSnapshot{}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// df
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := exec.CommandContext(ctx, "df", "-h", "/").Output()
		if err == nil {
			entries := parseDF(string(out))
			mu.Lock()
			snap.Disk = entries
			mu.Unlock()
		}
	}()

	// free
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := exec.CommandContext(ctx, "free", "-h").Output()
		if err == nil {
			mem := parseFree(string(out))
			mu.Lock()
			snap.Memory = mem
			mu.Unlock()
		}
	}()

	// uptime
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := exec.CommandContext(ctx, "uptime").Output()
		if err == nil {
			mu.Lock()
			snap.Uptime = strings.TrimSpace(string(out))
			mu.Unlock()
		}
	}()

	// loadavg
	wg.Add(1)
	go func() {
		defer wg.Done()
		data, err := os.ReadFile("/proc/loadavg")
		if err == nil {
			mu.Lock()
			snap.LoadAvg = strings.TrimSpace(string(data))
			mu.Unlock()
		}
	}()

	wg.Wait()
	_ = t0
	return snap, nil
}

func parseDF(output string) []DiskEntry {
	var entries []DiskEntry
	lines := strings.Split(output, "\n")
	for _, line := range lines[1:] { // skip header
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			entries = append(entries, DiskEntry{
				Filesystem: fields[0],
				Size:       fields[1],
				Used:       fields[2],
				Available:  fields[3],
				UsePercent: fields[4],
				MountPoint:  fields[5],
			})
		}
	}
	return entries
}

func parseFree(output string) MemoryInfo {
	var m MemoryInfo
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if strings.HasPrefix(line, "Mem:") && len(fields) >= 7 {
			m.Total = fields[1]
			m.Used = fields[2]
			m.Free = fields[3]
			m.Available = fields[6]
		}
	}
	return m
}
