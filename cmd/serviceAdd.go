package cmd

import (
	"errors"
	"fmt"

	nanoid "github.com/aidarkhanov/nanoid/v2"
	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "add a services by its name and get a token",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("rm requires only a service name")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		migrate()
		token, err := nanoid.GenerateString(nanoid.DefaultAlphabet, DefaultSize)
		if err != nil {
			panic(fmt.Errorf("%v", err))
		}
		db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
		if err != nil {
			panic(fmt.Errorf("%v", err))
		}
		service := Service{Token: token, ServiceName: args[0]}
		addResult := db.Create(&service)
		if addResult.RowsAffected != 1 {
			panic(fmt.Errorf("got error with %v rows affected", addResult.RowsAffected))
		}
		fmt.Println(token)
	},
}

func init() {
	serviceCmd.AddCommand(addCmd)
}
