package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/0embsd/devrec/internal/collectors"
	"github.com/0embsd/devrec/internal/session"
	"github.com/0embsd/devrec/internal/snapshot"
	"github.com/0embsd/devrec/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	watchCmd.Flags().DurationP("interval", "i", 30*time.Second, "snapshot interval")
	watchCmd.Flags().DurationP("duration", "d", 0, "total duration (0 = until interrupt)")
	watchCmd.Flags().StringP("collectors", "c", "", "collectors to use")
	watchCmd.Flags().StringP("tag", "t", "", "session label")
	rootCmd.AddCommand(watchCmd)
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch mode: periodic snapshots without terminal recording",
	RunE:  runWatch,
}

func runWatch(cmd *cobra.Command, args []string) error {
	c := getConfig()

	interval, _ := cmd.Flags().GetDuration("interval")
	duration, _ := cmd.Flags().GetDuration("duration")
	tag, _ := cmd.Flags().GetString("tag")

	collectorNames := c.DefaultCollectors
	if v, _ := cmd.Flags().GetString("collectors"); v != "" {
		collectorNames = parseCollectorList(v)
	}

	st, err := storage.NewManager(c.BaseDir)
	if err != nil {
		return err
	}

	sm := session.NewManager(st.TempDir(), c.PIDFile())
	meta, err := sm.Create(tag, collectorNames)
	if err != nil {
		return err
	}

	reg := collectors.DefaultRegistry()
	engine := snapshot.NewEngine(reg, c.CollectorTimeout)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		cancel()
	}()

	var deadline <-chan time.Time
	if duration > 0 {
		t := time.NewTimer(duration)
		defer t.Stop()
		deadline = t.C
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Printf("=== devrec watch ===\n")
	fmt.Printf("Session:   %s\n", meta.ID)
	fmt.Printf("Interval:  %s\n", interval)
	if duration > 0 {
		fmt.Printf("Duration:  %s\n", duration)
	}
	fmt.Printf("Collectors: %v\n", collectorNames)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	seq := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nStopping watch...")
			goto done
		case <-deadline:
			fmt.Println("\nDuration reached.")
			goto done
		case <-ticker.C:
			seq++
			label := fmt.Sprintf("snap-%03d", seq)
			snap := engine.Run(ctx, meta.ID, label, collectorNames, nil)
			snapPath := filepath.Join(sm.ActiveDir(), fmt.Sprintf("snap-%03d.json", seq))
			_ = snapshot.WriteJSON(snap, snapPath)
			fmt.Printf("  [%d] %s\n", seq, snapPath)
		}
	}

done:
	archive, err := st.Archive(meta.ID)
	if err != nil {
		return fmt.Errorf("archive: %w", err)
	}
	fmt.Printf("Archived: %s (%d bytes)\n", archive.Path, archive.SizeBytes)
	return nil
}
