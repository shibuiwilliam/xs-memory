package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Completely delete a memory store directory",
	Long:  "Removes the entire store directory and all its contents. Requires --store to be explicitly set.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("store") {
			return fmt.Errorf("--store flag is required for clean (to prevent accidental deletion)")
		}

		info, err := os.Stat(storePath)
		if err != nil {
			return fmt.Errorf("store not found: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", storePath)
		}

		if err := os.RemoveAll(storePath); err != nil {
			return fmt.Errorf("remove store: %w", err)
		}

		fmt.Printf("Deleted store: %s\n", storePath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
