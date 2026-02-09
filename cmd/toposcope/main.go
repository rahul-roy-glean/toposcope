// Package main provides the toposcope CLI entry point.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "toposcope",
		Short: "Structural intelligence for Bazel codebases",
		Long: `Toposcope extracts build dependency graphs from Bazel, computes deltas
between commits, and scores structural health.`,
		Version: version,
	}

	rootCmd.AddCommand(
		newSnapshotCmd(),
		newDiffCmd(),
		newScoreCmd(),
		newUICmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
