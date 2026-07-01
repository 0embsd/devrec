package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/0embsd/devrec/internal/collectors"
	"github.com/0embsd/devrec/internal/recorder"
	"github.com/0embsd/devrec/internal/session"
	"github.com/0embsd/devrec/internal/snapshot"
	"github.com/0embsd/devrec/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	startCmd.Flags().StringP("tag", "t", "", "label for this session")
	startCmd.Flags().StringP("collectors", "c", "", "collectors to enable (comma-separated)")
	startCmd.Flags().StringP("shell", "s", "", "shell for recording")
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a test recording session",
	Long:  "Start a terminal recording session with pre/post system snapshots.",
	RunE:  runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	c := getConfig()

	active, info, err := session.IsActive(c.PIDFile())
	if err != nil {
		return fmt.Errorf("check active session: %w", err)
	}
	if active {
		return fmt.Errorf("session %s already active (pid %d). Stop it first: devrec stop", info.SessionID, info.PID)
	}
	session.CleanStalePID(c.PIDFile())

	tag, _ := cmd.Flags().GetString("tag")
	shell, _ := cmd.Flags().GetString("shell")
	if shell == "" {
		shell = c.Shell
	}

	collectorNames := c.DefaultCollectors
	if v, _ := cmd.Flags().GetString("collectors"); v != "" {
		collectorNames = parseCollectorList(v)
	}

	st, err := storage.NewManager(c.BaseDir)
	if err != nil {
		return fmt.Errorf("storage init: %w", err)
	}

	sm := session.NewManager(st.TempDir(), c.PIDFile())
	meta, err := sm.Create(tag, collectorNames)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	reg := collectors.DefaultRegistry()
	todo, err := collectors.FilterRegistry(reg, collectorNames)
	if err != nil {
		return err
	}
	_ = todo

	engine := snapshot.NewEngine(reg, c.CollectorTimeout)

	fmt.Println("=== devrec start ===")
	fmt.Printf("Session:  %s\n", meta.ID)
	if tag != "" {
		fmt.Printf("Tag:      %s\n", tag)
	}
	fmt.Printf("Collectors: %v\n", collectorNames)
	fmt.Println()

	fmt.Print("Pre-snapshot... ")
	pre := engine.Run(context.Background(), meta.ID, "pre", collectorNames, nil)
	prePath := filepath.Join(sm.ActiveDir(), "pre.json")
	if err := snapshot.WriteJSON(pre, prePath); err != nil {
		return fmt.Errorf("write pre-snapshot: %w", err)
	}
	fmt.Println("OK")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "\n[devrec] received %v, shutting down...\n", sig)
		cancel()
	}()

	logFile := filepath.Join(sm.ActiveDir(), "terminal.log")
	timingFile := filepath.Join(sm.ActiveDir(), "terminal.time")
	rec := recorder.NewRecorder(shell, logFile, timingFile)

	fmt.Println("Recording started. Type 'exit' or Ctrl+D to stop.")
	fmt.Println()

	scriptPID, recErr := rec.Start(ctx)

	signal.Stop(sigCh)
	close(sigCh)

	fmt.Println()
	fmt.Print("Post-snapshot... ")
	post := engine.Run(context.Background(), meta.ID, "post", collectorNames, nil)
	postPath := filepath.Join(sm.ActiveDir(), "post.json")
	if err := snapshot.WriteJSON(post, postPath); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: write post-snapshot: %v\n", err)
	} else {
		fmt.Println("OK")
	}

	fmt.Print("Computing diff... ")
	diff := snapshot.ComputeDiff(pre, post)
	diffPath := filepath.Join(sm.ActiveDir(), "diff.json")
	if err := snapshot.WriteJSONRaw(diff, diffPath); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: write diff: %v\n", err)
	} else {
		fmt.Println("OK")
	}

	sm.SetPID(scriptPID)
	session.RemovePIDFile(c.PIDFile())

	fmt.Print("Packaging... ")
	archive, err := st.Archive(meta.ID)
	if err != nil {
		return fmt.Errorf("archive: %w", err)
	}
	fmt.Println("OK")
	fmt.Println()
	fmt.Println("=== Recording complete ===")
	fmt.Printf("Archive:  %s\n", archive.Path)
	fmt.Printf("Size:     %d bytes\n", archive.SizeBytes)
	fmt.Printf("Replay:   devrec replay %s\n", meta.ID)

	if recErr != nil && recErr != context.Canceled {
		return fmt.Errorf("recording interrupted: %w", recErr)
	}
	return nil
}

func parseCollectorList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
