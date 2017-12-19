package cmd

import (
	"fmt"
	"named/api"
	"github.com/spf13/cobra"
	"os"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validates policy files",
	Long:  `Validates policy files`,
	Run: func(cmd *cobra.Command, args []string) {

		file, err := cmd.Flags().GetString("file")
		if err != nil {
			fmt.Printf("Error when getting flag: file. %v", err)
			os.Exit(1)
		}

		validationErrors := api.ValidatePolicyFiles([]string{file})
		if len(validationErrors.Errors) != 0 {
			fmt.Println("Found errors while validating policy files")
			fmt.Printf("%v", validationErrors)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringP("file", "f", "app-policies.xml", "path to file")
}
