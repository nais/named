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

		files, err := cmd.Flags().GetStringArray("files")
		if err != nil {
			fmt.Printf("Error when getting flag: files. %v", err)
			os.Exit(1)
		}

		validationErrors := api.ValidatePolicyFiles(files)
		if len(validationErrors.Errors) != 0 {
			fmt.Println("Found errors while validating policy files")
			fmt.Printf("%v", validationErrors)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringArray("files", []string{"app-policies.xml", "not-enforced-urls.txt"}, "path to files")
}