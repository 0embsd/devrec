package cmd

import (
	"fmt"

	"github.com/0embsd/devrec/internal/storage"
	"github.com/spf13/cobra"
)

func init() {
	listCmd.Flags().IntP("limit", "n", 10, "max entries to show")
	listCmd.Flags().Bool("json", false, "output as JSON")
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List historical sessions",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	c := getConfig()

	limit, _ := cmd.Flags().GetInt("limit")
	asJSON, _ := cmd.Flags().GetBool("json")

	st, err := storage.NewManager(c.BaseDir)
	if err != nil {
		return err
	}

	archives, err := st.List(limit)
	if err != nil {
		return err
	}

	if asJSON {
		for _, a := range archives {
			fmt.Printf(`{"id":"%s","tag":"%s","created_at":"%s","size_bytes":%d}\n`,
				a.ID, a.Tag, a.CreatedAt.Format("2006-01-02T15:04:05"), a.SizeBytes)
		}
		return nil
	}

	if len(archives) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	fmt.Println("ID                                    TAG        DATE             SIZE")
	fmt.Println("---                                   ---        ----             ----")
	for _, a := range archives {
		tag := a.Tag
		if tag == "" {
			tag = "-"
		}
		fmt.Printf("%-36s  %-8s  %-16s  %d\n",
			a.ID, tag, a.CreatedAt.Format("2006-01-02 15:04"), a.SizeBytes)
	}
	return nil
}
