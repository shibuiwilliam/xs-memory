package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var rmHard bool

var rmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Remove a memory (soft delete by default)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		if err := s.Forget(context.Background(), args[0], rmHard); err != nil {
			return err
		}

		kind := "soft"
		if rmHard {
			kind = "hard"
		}
		fmt.Printf("Deleted (%s): %s\n", kind, args[0])
		return nil
	},
}

func init() {
	rmCmd.Flags().BoolVar(&rmHard, "hard", false, "permanently delete (not just tombstone)")
	rootCmd.AddCommand(rmCmd)
}
