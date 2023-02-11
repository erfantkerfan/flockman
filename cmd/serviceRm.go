package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "remove a services by its name",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("rm requires only a service name")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		migrate()
		db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
		if err != nil {
			panic(fmt.Errorf("%v", err))
		}
		deleteResult := db.Where("service_name = ?", args[0]).Delete(&Service{})
		if deleteResult.RowsAffected != 1 {
			panic(fmt.Errorf("got error with %v rows affected", deleteResult.RowsAffected))
		}
	},
}

func init() {
	serviceCmd.AddCommand(rmCmd)
}
