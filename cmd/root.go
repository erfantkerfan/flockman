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
	Short: "flockman is responsible for updating you docker swarm services",
	Long: `flockman exposes apis to call updates for you docker services,
registering services can be done via a cli and data is stored in a SQLite database.`,
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
