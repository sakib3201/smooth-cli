package main

import (
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp serve",
	Short: "Start the MCP server on stdio",
	Long:  "Runs the MCP server on standard I/O, suitable for use as an MCP tool in Claude Desktop or Claude Code.",
	RunE:  func(cmd *cobra.Command, args []string) error { return nil },
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
