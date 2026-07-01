package storage

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Archive represents a completed session archive.
type Archive struct {
	ID        string    `json:"id"`
	Tag       string    `json:"tag,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
}

// Manager handles storage I/O for session archives.
type Manager struct {
	baseDir     string
	sessionsDir string
	tempDir     string
}

// NewManager creates a Manager and ensures directories exist.
func NewManager(baseDir string) (*Manager, error) {
	m := &Manager{
		baseDir:     baseDir,
		sessionsDir: filepath.Join(baseDir, "sessions"),
		tempDir:     filepath.Join(baseDir, "tmp"),
	}
	if err := os.MkdirAll(m.sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}
	if err := os.MkdirAll(m.tempDir, 0700); err != nil {
		return nil, fmt.Errorf("create tmp dir: %w", err)
	}
	return m, nil
}

// SessionsDir returns the completed sessions directory.
func (m *Manager) SessionsDir() string { return m.sessionsDir }

// TempDir returns the active sessions temp directory.
func (m *Manager) TempDir() string { return m.tempDir }

// CreateTempDir creates a temp directory for an active session.
func (m *Manager) CreateTempDir(id string) (string, error) {
	dir := filepath.Join(m.tempDir, id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create session temp dir: %w", err)
	}
	return dir, nil
}

// RemoveTempDir removes a session temp directory.
func (m *Manager) RemoveTempDir(id string) error {
	dir := filepath.Join(m.tempDir, id)
	return os.RemoveAll(dir)
}

// Archive creates a tar.gz from the active temp directory and moves it
// to the sessions/ directory.
func (m *Manager) Archive(sessionID string) (*Archive, error) {
	srcDir := filepath.Join(m.tempDir, sessionID)
	archiveName := sessionID + ".tar.gz"
	archivePath := filepath.Join(m.sessionsDir, archiveName)

	f, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("create archive: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	err = filepath.Walk(srcDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(tw, src)
		return err
	})
	if err != nil {
		f.Close()
		os.Remove(archivePath)
		return nil, fmt.Errorf("archive walk: %w", err)
	}

	if err := tw.Close(); err != nil {
		f.Close()
		os.Remove(archivePath)
		return nil, fmt.Errorf("tar close: %w", err)
	}
	if err := gw.Close(); err != nil {
		f.Close()
		os.Remove(archivePath)
		return nil, fmt.Errorf("gzip close: %w", err)
	}
	f.Close()

	// Remove the temp directory after successful archive.
	os.RemoveAll(srcDir)

	st, err := os.Stat(archivePath)
	if err != nil {
		return nil, err
	}

	info := &Archive{
		ID:        sessionID,
		CreatedAt: st.ModTime(),
		Path:      archivePath,
		SizeBytes: st.Size(),
	}

	// Try to read tag from the archived session.json.
	if meta, err := readSessionMeta(archivePath); err == nil && meta.Tag != "" {
		info.Tag = meta.Tag
	}

	return info, nil
}

type sessionMeta struct {
	Tag string `json:"tag"`
}

func readSessionMeta(archivePath string) (sessionMeta, error) {
	// Read session.json from inside the tar.gz.
	f, err := os.Open(archivePath)
	if err != nil {
		return sessionMeta{}, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return sessionMeta{}, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if hdr.Name == "session.json" || strings.HasSuffix(hdr.Name, "/session.json") {
			data, err := io.ReadAll(tr)
			if err != nil {
				return sessionMeta{}, err
			}
			// Manual parse: find "tag"
			var m sessionMeta
			content := string(data)
			if idx := strings.Index(content, `"tag"`); idx >= 0 {
				// Find the value after "tag":
				rest := content[idx+5:]
				if ci := strings.Index(rest, `"`); ci >= 0 {
					rest = rest[ci+1:]
					if cj := strings.Index(rest, `"`); cj >= 0 {
						m.Tag = rest[:cj]
					}
				}
			}
			return m, nil
		}
	}
	return sessionMeta{}, nil
}

// List returns all completed session archives, newest first.
func (m *Manager) List(limit int) ([]Archive, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var archives []Archive
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tar.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".tar.gz")
		p := filepath.Join(m.sessionsDir, e.Name())
		a := Archive{
			ID:        id,
			CreatedAt: info.ModTime(),
			Path:      p,
			SizeBytes: info.Size(),
		})
	}

	sort.Slice(archives, func(i, j int) bool {
		return archives[i].CreatedAt.After(archives[j].CreatedAt)
	})

	if limit > 0 && len(archives) > limit {
		archives = archives[:limit]
	}
	return archives, nil
}

// Cleanup removes old archives, keeping at most `keep` most recent.
func (m *Manager) Cleanup(keep int, dryRun bool) ([]string, error) {
	if keep <= 0 {
		return nil, nil
	}
	all, err := m.List(0)
	if err != nil {
		return nil, err
	}
	if len(all) <= keep {
		return nil, nil
	}

	var removed []string
	for _, a := range all[keep:] {
		removed = append(removed, a.Path)
		if !dryRun {
			os.Remove(a.Path)
		}
	}
	return removed, nil
}

// GetArchive returns info for a specific archive by ID.
func (m *Manager) GetArchive(id string) (*Archive, error) {
	p := filepath.Join(m.sessionsDir, id+".tar.gz")
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	return &Archive{
		ID:        id,
		CreatedAt: st.ModTime(),
		Path:      p,
		SizeBytes: st.Size(),
	}, nil
}

// ExtractTemp extracts an archive to a temp directory for replay.
func (m *Manager) ExtractTemp(id string) (string, error) {
	archivePath := filepath.Join(m.sessionsDir, id+".tar.gz")
	destDir := filepath.Join(m.tempDir, id+".replay")
	os.RemoveAll(destDir)
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return "", err
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		target := filepath.Join(destDir, hdr.Name)
		if hdr.FileInfo().IsDir() {
			os.MkdirAll(target, 0700)
			continue
		}
		os.MkdirAll(filepath.Dir(target), 0700)
		out, err := os.Create(target)
		if err != nil {
			return "", err
		}
		io.Copy(out, tr)
		out.Close()
	}
	return destDir, nil
}

// Fix: List now reads tags.
func (m *Manager) fixTag(a *Archive) {
	// Try to load tag from tar.gz.
	if meta, err := readSessionMeta(a.Path); err == nil && meta.Tag != "" {
		a.Tag = meta.Tag
	}
}
