package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/xs-memory/xs-memory/xsmem"
)

var (
	updateContent    string
	updateImportance float32
	updateType       string
)

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a memory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		patch := xsmem.UpdateOpts{}
		if cmd.Flags().Changed("content") {
			patch.Content = &updateContent
		}
		if cmd.Flags().Changed("importance") {
			patch.Importance = &updateImportance
		}
		if cmd.Flags().Changed("type") {
			t := xsmem.MemoryType(updateType)
			patch.Type = &t
		}

		if err := s.Update(context.Background(), args[0], patch); err != nil {
			return err
		}

		fmt.Printf("Updated: %s\n", args[0])
		return nil
	},
}

func init() {
	updateCmd.Flags().StringVar(&updateContent, "content", "", "new content")
	updateCmd.Flags().Float32Var(&updateImportance, "importance", 0, "new importance")
	updateCmd.Flags().StringVar(&updateType, "type", "", "new type")
	rootCmd.AddCommand(updateCmd)
}
