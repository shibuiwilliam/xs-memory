package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/xs-memory/xs-memory/xsmem"
)

var (
	initAnalyzer string
	initEmbedder string
	initEmbedDim int
)

var initCmd = &cobra.Command{
	Use:   "init [store]",
	Short: "Initialize a new store",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := storePath
		if len(args) > 0 {
			path = args[0]
		}

		s, err := xsmem.Open(path)
		if err != nil {
			return err
		}
		defer s.Close()

		// Create the default collection.
		err = s.CreateCollection(collection, initAnalyzer, initEmbedder, initEmbedDim)
		if err != nil {
			return err
		}

		fmt.Printf("Initialized store at %s\n", path)
		fmt.Printf("Collection: %s (analyzer=%s, embedder=%s, dim=%d)\n",
			collection, initAnalyzer, initEmbedder, initEmbedDim)
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&initAnalyzer, "analyzer", "en", "analyzer: en, ja, bigram")
	initCmd.Flags().StringVar(&initEmbedder, "embedder", "", "embedder ID")
	initCmd.Flags().IntVar(&initEmbedDim, "dim", 0, "embedding dimension")
	rootCmd.AddCommand(initCmd)
}
