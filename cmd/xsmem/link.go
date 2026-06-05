package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/xs-memory/xs-memory/xsmem"
)

var linkCmd = &cobra.Command{
	Use:   "link <subject> <predicate> <object>",
	Short: "Create a graph edge between entities",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		err = s.Link(context.Background(), xsmem.Triple{
			Subject:   args[0],
			Predicate: args[1],
			Object:    args[2],
			Weight:    1.0,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Linked: %s -[%s]-> %s\n", args[0], args[1], args[2])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(linkCmd)
}
