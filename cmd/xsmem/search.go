package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xs-memory/xs-memory/xsmem"
)

var (
	searchMode string
	searchTopK int
	searchJSON bool
)

var searchCmd = &cobra.Command{
	Use:   "search [store] <query>",
	Short: "Search memories",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[len(args)-1]

		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		ctx := context.Background()

		var mode xsmem.SearchMode
		switch searchMode {
		case "fts":
			mode = xsmem.FTS
		case "vector":
			mode = xsmem.Vector
		case "hybrid":
			mode = xsmem.Hybrid
		default:
			mode = xsmem.Hybrid
		}

		results, err := s.Search(ctx, xsmem.SearchOpts{
			Collection: collection,
			Text:       query,
			Mode:       mode,
			TopK:       searchTopK,
		})
		if err != nil {
			return err
		}

		if searchJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		for i, r := range results {
			fmt.Printf("[%d] ID: %s  Score: %.4f  Type: %s\n", i+1, r.Memory.ID, r.Score, r.Memory.Type)
			content := r.Memory.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			fmt.Printf("    %s\n\n", content)
		}
		return nil
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchMode, "mode", "hybrid", "search mode: fts, vector, hybrid")
	searchCmd.Flags().IntVar(&searchTopK, "topk", 10, "number of results")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(searchCmd)
}
