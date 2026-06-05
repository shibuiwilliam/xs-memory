package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xs-memory/xs-memory/xsmem"
)

var (
	addType       string
	addSource     string
	addImportance float32
)

var addCmd = &cobra.Command{
	Use:   "add [store] [file... | -]",
	Short: "Add memories to the store",
	Args:  cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		ctx := context.Background()

		// Read from stdin or files.
		var contents []struct {
			text   string
			source string
		}

		if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
			// Read from stdin.
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			contents = append(contents, struct {
				text   string
				source string
			}{text: string(data), source: "stdin"})
		} else {
			for _, path := range args {
				data, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("read %s: %w", path, err)
				}
				contents = append(contents, struct {
					text   string
					source string
				}{text: string(data), source: "file://" + path})
			}
		}

		for _, c := range contents {
			src := addSource
			if src == "" {
				src = c.source
			}
			id, err := s.Remember(ctx, xsmem.RememberOpts{
				Collection:  collection,
				Content:     strings.TrimSpace(c.text),
				ContentType: "text/plain",
				Source:      src,
				Type:        xsmem.MemoryType(addType),
				Importance:  addImportance,
			})
			if err != nil {
				return fmt.Errorf("remember: %w", err)
			}
			fmt.Printf("Added: %s (source=%s)\n", id, src)
		}
		return nil
	},
}

func init() {
	addCmd.Flags().StringVar(&addType, "type", "semantic", "memory type: episodic, semantic, procedural")
	addCmd.Flags().StringVar(&addSource, "source", "", "source identifier")
	addCmd.Flags().Float32Var(&addImportance, "importance", 0.5, "importance score 0..1")
	rootCmd.AddCommand(addCmd)
}
