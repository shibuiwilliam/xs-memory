package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var organizeJobs string

var organizeCmd = &cobra.Command{
	Use:   "organize",
	Short: "Run LLM organization jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		var jobs []string
		if organizeJobs != "" {
			jobs = strings.Split(organizeJobs, ",")
		}

		if err := s.Organize(context.Background(), collection, jobs...); err != nil {
			return err
		}
		fmt.Println("Organization complete.")
		return nil
	},
}

func init() {
	organizeCmd.Flags().StringVar(&organizeJobs, "jobs", "", "comma-separated jobs: extract,autotag,importance,dedup")
	rootCmd.AddCommand(organizeCmd)
}
