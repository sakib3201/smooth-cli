package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "smooth",
	Short: "smooth-cli: terminal dashboard for AI agent processes",
	Long:  "A TUI-based process supervisor that spawns, monitors, and manages AI agent processes with a Bubble Tea dashboard, REST/WebSocket API, and MCP server.",
	RunE:  func(cmd *cobra.Command, args []string) error { return nil },
}

var configFlag string
var portFlag int
var logLevelFlag string

func init() {
	rootCmd.PersistentFlags().StringVar(&configFlag, "config", "smooth.yml", "path to config file (YAML or TOML)")
	rootCmd.PersistentFlags().IntVar(&portFlag, "port", 7700, "REST + WebSocket API port (0 to disable)")
	rootCmd.PersistentFlags().StringVar(&logLevelFlag, "log-level", "info", "log verbosity: debug, info, warn, error")
}
