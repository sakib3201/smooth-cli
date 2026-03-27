package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var version = "1.0.0"
var commit = "unknown"
var date = "unknown"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("smooth-cli %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("date: %s\n", date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
