package storage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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

// zstdAvailable checks if the zstd binary is usable.
var zstdAvailable = checkZstd()

func checkZstd() bool {
	return exec.Command("zstd", "--version").Run() == nil
}

// zstdCompress compresses a file with zstd -3 -T0.
func zstdCompress(input, output string) error {
	cmd := exec.Command("zstd", "-3", "-T0", "-q", "-f", "-o", output, input)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// zstdDecompress decompresses a zst file to the given output.
func zstdDecompress(input, output string) error {
	cmd := exec.Command("zstd", "-d", "-q", "-f", "-o", output, input)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// getArchiveExt returns the archive file extension and compressor name.
func getArchiveExt() (ext string) {
	if zstdAvailable {
		return ".tar.zst"
	}
	return ".tar.gz"
}

// readTagFromTemp reads the tag from session.json in the temp directory.
func readTagFromTemp(srcDir string) string {
	data, err := os.ReadFile(filepath.Join(srcDir, "session.json"))
	if err != nil {
		return ""
	}
	var m struct {
		Tag string `json:"tag"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	return m.Tag
}

// Archive creates a tar.zst (or tar.gz fallback) from the active temp directory
// and moves it to the sessions/ directory. Also writes a .tag sidecar file.
func (m *Manager) Archive(sessionID string) (*Archive, error) {
	srcDir := filepath.Join(m.tempDir, sessionID)
	ext := getArchiveExt()
	archivePath := filepath.Join(m.sessionsDir, sessionID+ext)

	// Read tag from temp dir session.json before archiving.
	tag := readTagFromTemp(srcDir)

	// 1. Create uncompressed tar.
	tmpTar := archivePath + ".tar"
	tf, err := os.Create(tmpTar)
	if err != nil {
		return nil, fmt.Errorf("create tar: %w", err)
	}

	tw := tar.NewWriter(tf)
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
	closeErr := tw.Close()
	tf.Close()

	if err != nil {
		os.Remove(tmpTar)
		return nil, fmt.Errorf("archive walk: %w", err)
	}
	if closeErr != nil {
		os.Remove(tmpTar)
		return nil, fmt.Errorf("tar close: %w", closeErr)
	}

	// 2. Compress.
	if zstdAvailable {
		if err := zstdCompress(tmpTar, archivePath); err != nil {
			os.Remove(tmpTar)
			return nil, fmt.Errorf("zstd compress: %w", err)
		}
	} else {
		if err := gzipCompress(tmpTar, archivePath); err != nil {
			os.Remove(tmpTar)
			return nil, fmt.Errorf("gzip compress: %w", err)
		}
	}
	os.Remove(tmpTar)

	// 3. Remove the temp directory.
	os.RemoveAll(srcDir)

	// 4. Write .tag sidecar for fast tag lookup in List().
	if tag != "" {
		os.WriteFile(archivePath+".tag", []byte(tag), 0644)
	}

	st, err := os.Stat(archivePath)
	if err != nil {
		return nil, err
	}

	return &Archive{
		ID:        sessionID,
		Tag:       tag,
		CreatedAt: st.ModTime(),
		Path:      archivePath,
		SizeBytes: st.Size(),
	}, nil
}

// gzipCompress is the fallback compressor when zstd is unavailable.
func gzipCompress(input, output string) error {
	in, err := os.Open(input)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	_, err = io.Copy(gw, in)
	if closeErr := gw.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}

// findArchive returns the archive path for a session ID,
// trying .tar.zst first, then .tar.gz.
func (m *Manager) findArchive(id string) (string, error) {
	for _, ext := range []string{".tar.zst", ".tar.gz"} {
		p := filepath.Join(m.sessionsDir, id+ext)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("archive not found: %s", id)
}

// isArchiveExt returns true if the filename has a valid archive extension.
func isArchiveExt(name string) bool {
	return strings.HasSuffix(name, ".tar.zst") || strings.HasSuffix(name, ".tar.gz")
}

// archiveID extracts the session ID from an archive filename.
func archiveID(name string) string {
	name = strings.TrimSuffix(name, ".tar.zst")
	name = strings.TrimSuffix(name, ".tar.gz")
	return name
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
		if e.IsDir() || !isArchiveExt(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		id := archiveID(e.Name())
		p := filepath.Join(m.sessionsDir, e.Name())

		tag := ""
		if tagData, err := os.ReadFile(p + ".tag"); err == nil {
			tag = string(bytes.TrimSpace(tagData))
		}

		archives = append(archives, Archive{
			ID:        id,
			Tag:       tag,
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
			os.Remove(a.Path + ".tag")
		}
	}
	return removed, nil
}

// GetArchive returns info for a specific archive by ID.
func (m *Manager) GetArchive(id string) (*Archive, error) {
	p, err := m.findArchive(id)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	tag := ""
	if tagData, err := os.ReadFile(p + ".tag"); err == nil {
		tag = string(bytes.TrimSpace(tagData))
	}
	return &Archive{
		ID:        id,
		Tag:       tag,
		CreatedAt: st.ModTime(),
		Path:      p,
		SizeBytes: st.Size(),
	}, nil
}

// ExtractTemp extracts an archive to a temp directory for replay.
func (m *Manager) ExtractTemp(id string) (string, error) {
	archivePath, err := m.findArchive(id)
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(m.tempDir, id+".replay")
	os.RemoveAll(destDir)
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return "", err
	}

	// Decompress to temp tar.
	tmpTar := filepath.Join(m.tempDir, id+".tmp.tar")
	var tarPath string

	if strings.HasSuffix(archivePath, ".tar.zst") {
		if err := zstdDecompress(archivePath, tmpTar); err != nil {
			return "", fmt.Errorf("zstd decompress: %w", err)
		}
		tarPath = tmpTar
	} else {
		// gzip: decompress inline.
		tarPath = tmpTar
		if err := gunzipFile(archivePath, tmpTar); err != nil {
			return "", fmt.Errorf("gzip decompress: %w", err)
		}
	}
	defer os.Remove(tmpTar)

	// Untar.
	f, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	tr := tar.NewReader(f)
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

// gunzipFile decompresses a gzip file to the output path.
func gunzipFile(input, output string) error {
	in, err := os.Open(input)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	gz, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gz.Close()

	_, err = io.Copy(out, gz)
	return err
}
