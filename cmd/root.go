package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	DatabaseFile string
	version      string
)
var rootCmd = &cobra.Command{
	Use:   "flockman",
	Short: "flockman is responsible for updating your docker swarm services",
	Long: `flockman exposes APIs to call updates for your docker services,
registering services can be done via a CLI and data is stored in a SQLite database.`,
	Version: version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&DatabaseFile, "database", "D", "flockman.sqlite3", "db path")
}
