package session

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// State represents the current state of a session.
type State string

const (
	StateActive    State = "active"
	StateStopping  State = "stopping"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
)

// Meta is the JSON-serializable session metadata written to session.json.
type Meta struct {
	ID        string    `json:"id"`
	Tag       string    `json:"tag,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Hostname  string    `json:"hostname"`
	User      string    `json:"user"`
	CWD       string    `json:"cwd"`
	PID       int       `json:"pid"`
	State     State     `json:"state"`
	Collectors []string `json:"collectors,omitempty"`
}

// Manager manages the lifecycle of a single active session.
type Manager struct {
	tempDir   string
	pidFile   string
	meta      *Meta
	metaPath  string
	activeDir string
}

// NewManager creates a session Manager.
func NewManager(tempDir string, pidFile string) *Manager {
	return &Manager{
		tempDir: tempDir,
		pidFile: pidFile,
	}
}

// Create generates a new session.
func (m *Manager) Create(tag string, collectors []string) (*Meta, error) {
	id, err := generateUUID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()
	user := os.Getenv("USER")
	if user == "" {
		user = "root"
	}

	m.activeDir = filepath.Join(m.tempDir, id)
	if err := os.MkdirAll(m.activeDir, 0700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	m.meta = &Meta{
		ID:         id,
		Tag:        tag,
		StartedAt:  time.Now(),
		Hostname:   hostname,
		User:       user,
		CWD:        cwd,
		State:      StateActive,
		Collectors: collectors,
	}

	m.metaPath = filepath.Join(m.activeDir, "session.json")
	if err := m.writeMeta(); err != nil {
		return nil, err
	}

	return m.meta, nil
}

// Load loads session metadata from an existing session directory.
func (m *Manager) Load(id string) (*Meta, error) {
	m.activeDir = filepath.Join(m.tempDir, id)
	m.metaPath = filepath.Join(m.activeDir, "session.json")

	data, err := os.ReadFile(m.metaPath)
	if err != nil {
		return nil, fmt.Errorf("read session meta: %w", err)
	}
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse session meta: %w", err)
	}
	m.meta = &meta
	return &meta, nil
}

// ActiveID returns the current session ID.
func (m *Manager) ActiveID() string {
	if m.meta == nil {
		return ""
	}
	return m.meta.ID
}

// ActiveDir returns the current session temp directory.
func (m *Manager) ActiveDir() string {
	return m.activeDir
}

// Meta returns the current session metadata.
func (m *Manager) Meta() *Meta {
	return m.meta
}

// UpdateState updates the session state in memory and on disk.
func (m *Manager) UpdateState(state State) error {
	if m.meta == nil {
		return fmt.Errorf("no active session")
	}
	m.meta.State = state
	return m.writeMeta()
}

// SetPID records the script process PID and writes the PID file.
func (m *Manager) SetPID(pid int) error {
	if m.meta == nil {
		return fmt.Errorf("no active session")
	}
	m.meta.PID = pid
	if err := m.writeMeta(); err != nil {
		return err
	}

	// Write PID file: <pid>:<uuid>:<unix_ts>
	pidDir := filepath.Dir(m.pidFile)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("create pid dir: %w", err)
	}
	content := fmt.Sprintf("%d:%s:%d\n", pid, m.meta.ID, m.meta.StartedAt.Unix())
	return os.WriteFile(m.pidFile, []byte(content), 0644)
}

// ReadPIDFile reads the PID file and returns parsed values.
func ReadPIDFile(path string) (pid int, sessionID string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 3)
	if len(parts) < 2 {
		return 0, "", fmt.Errorf("invalid pid file format")
	}
	pid, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid pid: %w", err)
	}
	sessionID = parts[1]
	return pid, sessionID, nil
}

// IsActive checks if a session is currently active by reading the PID file
// and verifying the process is alive.
func IsActive(pidFile string) (bool, *ActiveInfo, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, err
	}

	parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 3)
	if len(parts) < 2 {
		return false, nil, nil
	}

	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		return false, nil, nil // corrupt PID file, treat as inactive
	}

	// Check if process is alive.
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, nil, nil
	}
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		return false, nil, nil // process not alive
	}

	info := &ActiveInfo{
		PID:       pid,
		SessionID: parts[1],
	}
	if len(parts) == 3 {
		ts, err := strconv.ParseInt(parts[2], 10, 64)
		if err == nil {
			t := time.Unix(ts, 0)
			info.StartedAt = &t
		}
	}
	return true, info, nil
}

// ActiveInfo holds info about an active session from the PID file.
type ActiveInfo struct {
	PID       int
	SessionID string
	StartedAt *time.Time
}

// RemovePIDFile removes the PID file.
func RemovePIDFile(pidFile string) error {
	err := os.Remove(pidFile)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// CleanStalePID removes the PID file if the process is dead.
func CleanStalePID(pidFile string) (cleaned bool, _ error) {
	data, err := os.ReadFile(pidFile)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 2)
	if len(parts) < 1 {
		os.Remove(pidFile)
		return true, nil
	}
	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		os.Remove(pidFile)
		return true, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(pidFile)
		return true, nil
	}
	if proc.Signal(syscall.Signal(0)) != nil {
		os.Remove(pidFile)
		return true, nil
	}
	return false, nil
}

// writeMeta writes current metadata to session.json atomically.
func (m *Manager) writeMeta() error {
	if m.metaPath == "" || m.meta == nil {
		return fmt.Errorf("session not initialized")
	}
	data, err := json.MarshalIndent(m.meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	tmp := m.metaPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}
	return os.Rename(tmp, m.metaPath)
}

// generateUUID creates a UUID v4 string using crypto/rand.
func generateUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// Set version 4.
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant 1 (RFC 4122).
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
