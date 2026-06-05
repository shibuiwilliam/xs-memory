package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xs-memory/xs-memory/xsmem"
)

var exportCmd = &cobra.Command{
	Use:   "export <archive.tar.gz>",
	Short: "Export store to archive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		f, err := os.Create(args[0])
		if err != nil {
			return fmt.Errorf("create archive: %w", err)
		}
		defer f.Close()

		if err := s.Export(f); err != nil {
			return err
		}
		fmt.Printf("Exported to %s\n", args[0])
		return nil
	},
}

var importCmd = &cobra.Command{
	Use:   "import <archive.tar.gz>",
	Short: "Import store from archive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("open archive: %w", err)
		}
		defer f.Close()

		if err := xsmem.Import(storePath, f); err != nil {
			return err
		}
		fmt.Printf("Imported to %s\n", storePath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
}
