package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var getJSON bool

var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a memory by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		mem, err := s.Get(context.Background(), args[0])
		if err != nil {
			return err
		}

		if getJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(mem)
		}

		fmt.Printf("ID:         %s\n", mem.ID)
		fmt.Printf("Collection: %s\n", mem.Collection)
		fmt.Printf("Type:       %s\n", mem.Type)
		fmt.Printf("Importance: %.2f\n", mem.Importance)
		fmt.Printf("Created:    %s\n", mem.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated:    %s\n", mem.UpdatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Content:\n%s\n", mem.Content)
		return nil
	},
}

func init() {
	getCmd.Flags().BoolVar(&getJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(getCmd)
}
