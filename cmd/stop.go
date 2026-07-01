package cmd

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/0embsd/devrec/internal/session"
	"github.com/spf13/cobra"
)

func init() {
	stopCmd.Flags().StringP("session", "s", "", "session ID to stop")
	rootCmd.AddCommand(stopCmd)
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a recording session and package results",
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	c := getConfig()

	pid, sessionID, err := session.ReadPIDFile(c.PIDFile())
	if err != nil {
		return fmt.Errorf("no active session: %s", c.PIDFile())
	}

	sid, _ := cmd.Flags().GetString("session")
	if sid != "" && sid != sessionID {
		return fmt.Errorf("session mismatch: PID file says %s, you specified %s", sessionID, sid)
	}

	fmt.Printf("Stopping session %s (pid %d)...\n", sessionID, pid)
	_ = sid

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal process %d: %w", pid, err)
	}

	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			fmt.Println("Session stopped. Packaging should complete automatically.")
			return nil
		}
	}

	fmt.Println("Warning: process did not exit in time.")
	return nil
}
