package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var infoJSON bool

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show detailed store and collection information",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		info := s.Info()

		if infoJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		}

		fmt.Printf("Store\n")
		fmt.Printf("  Path:             %s\n", info.AbsPath)
		fmt.Printf("  Manifest version: %d\n", info.ManifestVersion)
		fmt.Printf("  Total memories:   %d\n", info.TotalMemories)
		fmt.Printf("  Disk usage:       %s\n", info.DiskUsage.Total)
		if info.DiskUsage.MetaDB > 0 {
			fmt.Printf("    meta.db:        %s\n", humanSize(info.DiskUsage.MetaDB))
		}
		if info.DiskUsage.WAL > 0 {
			fmt.Printf("    wal/:           %s\n", humanSize(info.DiskUsage.WAL))
		}
		if info.DiskUsage.Segments > 0 {
			fmt.Printf("    segments/:      %s\n", humanSize(info.DiskUsage.Segments))
		}
		if info.DiskUsage.Blobs > 0 {
			fmt.Printf("    blobs/:         %s\n", humanSize(info.DiskUsage.Blobs))
		}
		fmt.Println()

		fmt.Printf("Collections (%d)\n", len(info.Collections))
		for _, c := range info.Collections {
			fmt.Printf("  [%s]\n", c.Name)
			fmt.Printf("    Analyzer:       %s\n", c.Analyzer)
			if c.EmbedderID != "" {
				fmt.Printf("    Embedder:       %s (dim=%d)\n", c.EmbedderID, c.EmbedDimension)
			} else {
				fmt.Printf("    Embedder:       (none)\n")
			}
			fmt.Printf("    Memories:       %d\n", c.MemoryCount)
			fmt.Printf("    FTS indexed:    %d docs\n", c.FTSDocCount)
			fmt.Printf("    Vector indexed: %d docs\n", c.VecDocCount)
		}
		if len(info.Collections) == 0 {
			fmt.Printf("  (none)\n")
		}
		fmt.Println()

		fmt.Printf("Block Cache\n")
		fmt.Printf("  Capacity:         %.1f MB\n", info.BlockCache.CapacityMB)
		fmt.Printf("  Used:             %.1f MB (%d blocks)\n", info.BlockCache.UsedMB, info.BlockCache.Count)
		fmt.Println()

		fmt.Printf("Result Cache\n")
		fmt.Printf("  Entries:          %d\n", info.ResultCache.EntryCount)
		fmt.Printf("  Hits / Misses:    %d / %d\n", info.ResultCache.Hits, info.ResultCache.Misses)
		fmt.Printf("  Evictions:        %d\n", info.ResultCache.Evictions)
		fmt.Printf("  Invalidations:    %d\n", info.ResultCache.Invalidations)
		fmt.Println()

		fmt.Printf("Tuning\n")
		fmt.Printf("  Epoch:            %d\n", info.Tuning.Epoch)
		fmt.Printf("  Events:           %d\n", info.Tuning.EventCount)
		fmt.Printf("  Item priors:      %d\n", info.Tuning.PriorCount)
		fmt.Printf("  Affinities:       %d\n", info.Tuning.AffinityCount)

		return nil
	},
}

func humanSize(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
	)
	switch {
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func init() {
	infoCmd.Flags().BoolVar(&infoJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(infoCmd)
}
