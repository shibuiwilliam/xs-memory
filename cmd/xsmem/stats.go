package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var statsJSON bool

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show store statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		stats := s.Stats()

		if statsJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(stats)
		}

		fmt.Printf("Store:        %s\n", stats.Path)
		fmt.Printf("Collections:  %d\n", stats.Collections)
		fmt.Printf("Memories:     %d\n", stats.Memories)
		fmt.Printf("Block Cache:  %.1f MB / %.1f MB (%d blocks)\n",
			stats.BlockCacheStats.UsedMB, stats.BlockCacheStats.CapacityMB, stats.BlockCacheStats.Count)
		fmt.Printf("Result Cache: %d entries, %d hits, %d misses, %d evictions\n",
			stats.ResultCache.EntryCount, stats.ResultCache.Hits,
			stats.ResultCache.Misses, stats.ResultCache.Evictions)
		fmt.Printf("Tuning:       epoch=%d, events=%d, priors=%d, affinities=%d\n",
			stats.Tuning.Epoch, stats.Tuning.EventCount,
			stats.Tuning.PriorCount, stats.Tuning.AffinityCount)
		fmt.Printf("FTS Index:    %d terms, %d docs\n",
			stats.Structural.FTSTermCount, stats.Structural.FTSDocCount)
		fmt.Printf("Vector Index: %d vectors (dim=%d, quantize=%v)\n",
			stats.Structural.VectorCount, stats.Structural.VectorDim, stats.Structural.VectorQuantize)
		fmt.Printf("Graph:        %d edges\n", stats.Structural.GraphEdgeCount)
		fmt.Printf("Metrics:      enabled=%v\n", stats.MetricsEnabled)
		return nil
	},
}

func init() {
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(statsCmd)
}
