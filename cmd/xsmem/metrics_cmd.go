package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xs-memory/xs-memory/internal/metrics"
)

var (
	metricsJSON  bool
	metricsReset bool
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show or reset search metrics",
	Long:  "Display search metrics (counts, hit rate, mode distribution, latency) or reset them. See addendum3 §3.",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		if metricsReset {
			s.MetricsReset(collection)
			fmt.Println("Metrics reset.")
			return nil
		}

		if !s.MetricsEnabled() {
			fmt.Println("Metrics are disabled. Enable with metrics.enabled = true in config.")
			return nil
		}

		snap := s.MetricsSnapshot(collection)
		snap.ComputeModeDistribution()

		if metricsJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(snap)
		}

		// Human-readable output.
		fmt.Printf("Total searches:  %d\n", snap.TotalSearches)
		fmt.Printf("Fill rate:       %.2f\n", snap.FillRate)
		fmt.Println()

		fmt.Println("Per-mode:")
		for _, m := range metrics.AllModes {
			count := snap.SearchCounts[m]
			if count == 0 {
				continue
			}
			under := snap.Underfilled[m]
			ret := snap.Returned[m]
			req := snap.Requested[m]
			share := float64(0)
			if snap.ModeDistribution != nil {
				share = snap.ModeDistribution[m] * 100
			}
			fmt.Printf("  %-8s  count=%-6d  returned=%-6d  requested=%-6d  underfilled=%-4d  share=%.1f%%\n",
				m, count, ret, req, under, share)
		}

		if snap.Latency != nil {
			fmt.Println()
			fmt.Println("Latency histograms:")
			for _, m := range metrics.AllModes {
				ls := snap.Latency[m]
				if ls.Total == 0 {
					continue
				}
				fmt.Printf("  %s: ", m)
				for i, label := range metrics.BucketLabels {
					if ls.Buckets[i] > 0 {
						fmt.Printf("%s=%d ", label, ls.Buckets[i])
					}
				}
				fmt.Printf("(total=%d)\n", ls.Total)
			}
		}

		if len(snap.TopTerms) > 0 {
			fmt.Println()
			fmt.Println("Top search terms:")
			limit := 20
			if len(snap.TopTerms) < limit {
				limit = len(snap.TopTerms)
			}
			for _, te := range snap.TopTerms[:limit] {
				fmt.Printf("  %-30s  %.0f\n", te.Token, te.Count)
			}
		}

		return nil
	},
}

func init() {
	metricsCmd.Flags().BoolVar(&metricsJSON, "json", false, "output as JSON")
	metricsCmd.Flags().BoolVar(&metricsReset, "reset", false, "reset all metrics")
	rootCmd.AddCommand(metricsCmd)
}
