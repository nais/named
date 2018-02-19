package cmd

import (
	"fmt"
	"github.com/nais/named/api"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
)

var validateSbsCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validates policy files",
	Long:  `Validates policy files`,
	Run: func(cmd *cobra.Command, args []string) {

		file, err := cmd.Flags().GetString("file")
		if err != nil {
			fmt.Printf("Error when getting flag: file. %v", err)
			os.Exit(1)
		}

		output, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Printf("Could not read file: %s. %v", file, err)
			fmt.Printf("File contents: %s", output)
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
	RootCmd.AddCommand(validateSbsCmd)
	validateSbsCmd.Flags().StringP("file", "f", "app-policies.xml", "path to file")
}
