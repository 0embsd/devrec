package cmd

import (
	"fmt"

	"github.com/0embsd/devrec/internal/session"
	"github.com/0embsd/devrec/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active session status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	c := getConfig()

	active, info, err := session.IsActive(c.PIDFile())
	if err != nil {
		return fmt.Errorf("check status: %w", err)
	}

	if active && info != nil {
		fmt.Println("● Recording active")
		fmt.Printf("  Session: %s\n", info.SessionID)
		fmt.Printf("  PID:     %d\n", info.PID)
		if info.StartedAt != nil {
			fmt.Printf("  Since:   %s\n", info.StartedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
		fmt.Println("  Stop with: devrec stop")
	} else {
		fmt.Println("○ No active recording")
	}

	// Show recent archives.
	st, err := storage.NewManager(c.BaseDir)
	if err != nil {
		return err
	}
	archives, err := st.List(3)
	if err != nil {
		return err
	}
	if len(archives) > 0 {
		fmt.Println()
		fmt.Println("Recent sessions:")
		for _, a := range archives {
			tagStr := ""
			if a.Tag != "" {
				tagStr = " [" + a.Tag + "]"
			}
			fmt.Printf("  %s  %s%s  %d bytes\n",
				a.CreatedAt.Format("2006-01-02 15:04"),
				a.ID[:12],
				tagStr,
				a.SizeBytes,
			)
		}
	}

	return nil
}
