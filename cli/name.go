package main

import (
	"fmt"
	"os"

	cmd "github.com/nais/named/cli/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
