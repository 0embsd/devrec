package cmd

import (
	"fmt"

	"github.com/0embsd/devrec/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	cleanupCmd.Flags().IntP("keep", "k", 20, "number of recent sessions to keep")
	cleanupCmd.Flags().Bool("dry-run", false, "show what would be removed")
	rootCmd.AddCommand(cleanupCmd)
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove old session archives",
	RunE:  runCleanup,
}

func runCleanup(cmd *cobra.Command, args []string) error {
	c := getConfig()

	keep, _ := cmd.Flags().GetInt("keep")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	st, err := storage.NewManager(c.BaseDir)
	if err != nil {
		return err
	}

	removed, err := st.Cleanup(keep, dryRun)
	if err != nil {
		return err
	}

	if len(removed) == 0 {
		fmt.Println("Nothing to clean up.")
		return nil
	}

	action := "Removed"
	if dryRun {
		action = "Would remove"
	}
	for _, p := range removed {
		fmt.Printf("%s: %s\n", action, p)
	}
	fmt.Printf("%s %d archive(s), %d remaining.\n", action, len(removed), keep)
	return nil
}
