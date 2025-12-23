package cmd

import (
	"errors"
	"fmt"

	nanoid "github.com/aidarkhanov/nanoid/v2"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "add a services by its name and get a token",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("add requires exactly one service name")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := getDB()
		if err != nil {
			return err
		}
		token, err := nanoid.GenerateString(nanoid.DefaultAlphabet, DefaultSize)
		if err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}
		service := Service{Token: token, ServiceName: args[0]}
		addResult := db.Create(&service)
		if addResult.Error != nil {
			return fmt.Errorf("failed to add service: %w", addResult.Error)
		}
		if addResult.RowsAffected != 1 {
			return fmt.Errorf("unexpected rows affected: %d", addResult.RowsAffected)
		}
		fmt.Println(token)
		return nil
	},
}

func init() {
	serviceCmd.AddCommand(addCmd)
}
