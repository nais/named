package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

// Execute executes named CLI command
func Execute() {
	RootCmd.Execute()
}

// RootCmd uses cobra to execute desired command
var RootCmd = &cobra.Command{
	Use:   "name",
	Short: "name is the CLI for the nameD service",
	Long:  "name is the CLI for the nameD service",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("AM configuration service")
	},
}
