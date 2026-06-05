package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var lsJSON bool

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List memories in a collection",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		mems, err := s.List(context.Background(), collection)
		if err != nil {
			return err
		}

		if lsJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(mems)
		}

		if len(mems) == 0 {
			fmt.Println("No memories found.")
			return nil
		}

		for _, m := range mems {
			content := m.Content
			if len(content) > 80 {
				content = content[:80] + "..."
			}
			fmt.Printf("%s  %s  %.2f  %s\n", m.ID, m.Type, m.Importance, content)
		}
		fmt.Printf("\nTotal: %d memories\n", len(mems))
		return nil
	},
}

func init() {
	lsCmd.Flags().BoolVar(&lsJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(lsCmd)
}
