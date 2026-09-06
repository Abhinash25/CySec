// CySec.env — One Environment. Every Tool.
//
// Main entry point for the cysec CLI application.
package main

import (
	"fmt"
	"os"

	"github.com/cysec-env/cysec/internal/cli"
)

func main() {
	rootCmd := cli.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
