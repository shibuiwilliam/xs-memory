package main

import (
	"github.com/spf13/cobra"
)

var (
	storePath  string
	collection string
)

var rootCmd = &cobra.Command{
	Use:   "smem",
	Short: "small-memory: embedded memory engine for AI agents",
	Long:  "An embedded memory engine providing full-text, vector, and hybrid search for local AI agents.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&storePath, "store", "default.smem", "path to store directory")
	rootCmd.PersistentFlags().StringVarP(&collection, "collection", "c", "default", "collection name")
}
