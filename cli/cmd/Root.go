package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

func Execute() {
	RootCmd.Execute()
}

var RootCmd = &cobra.Command{
	Use:   "name",
	Short: "name is the CLI for the NameD AAAS",
	Long:  "name is the CLI for the NameD AAAS",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("AM configuration service")
	},
}
