package main

import (
	"github.com/spf13/cobra"

	mcpserver "github.com/xs-memory/xs-memory/interfaces/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdio)",
	Long:  "Start an MCP (Model Context Protocol) server using stdio transport. See design §13.2.",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openStore()
		if err != nil {
			return err
		}
		defer s.Close()

		srv := mcpserver.NewServer(s)
		return srv.ServeStdio()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
