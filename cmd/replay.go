package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/0embsd/devrec/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(replayCmd)
}

var replayCmd = &cobra.Command{
	Use:   "replay <session-id>",
	Short: "Replay a recorded terminal session",
	Args:  cobra.ExactArgs(1),
	RunE:  runReplay,
}

func runReplay(cmd *cobra.Command, args []string) error {
	c := getConfig()
	sessionID := args[0]

	// Check that scriptreplay is available.
	if _, err := exec.LookPath("scriptreplay"); err != nil {
		return fmt.Errorf("scriptreplay not found; install util-linux (apt install util-linux)")
	}

	st, err := storage.NewManager(c.BaseDir)
	if err != nil {
		return err
	}

	// Extract the archive to a temp dir.
	dir, err := st.ExtractTemp(sessionID)
	if err != nil {
		return fmt.Errorf("extract session %s: %w", sessionID, err)
	}
	defer os.RemoveAll(dir)

	logFile := filepath.Join(dir, "terminal.log")
	timingFile := filepath.Join(dir, "terminal.time")

	if _, err := os.Stat(timingFile); os.IsNotExist(err) {
		// No timing file; just cat the log.
		data, err := os.ReadFile(logFile)
		if err != nil {
			return fmt.Errorf("read terminal log: %w", err)
		}
		os.Stdout.Write(data)
		return nil
	}

	// Use scriptreplay for proper playback.
	replay := exec.Command("scriptreplay", "--timing="+timingFile, logFile)
	replay.Stdin = os.Stdin
	replay.Stdout = os.Stdout
	replay.Stderr = os.Stderr
	return replay.Run()
}
