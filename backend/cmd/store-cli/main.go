package main

import (
	"os"

	"github.com/SteVio89/stevio-home/cmd/store-cli/commands"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "store-cli",
		Short: "Stevio Store operator toolbox",
	}

	root.AddCommand(
		commands.GenerateCmd(),
		commands.HashEmailCmd(),
		commands.CheckConfigCmd(),
		commands.MigrateCmd(),
		commands.DBStatsCmd(),
		commands.SigningKeysCmd(),
		commands.CleanupCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
