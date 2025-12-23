package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "remove a services by its name",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("rm requires exactly one service name")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := getDB()
		if err != nil {
			return err
		}
		deleteResult := db.Where("service_name = ?", args[0]).Delete(&Service{})
		if deleteResult.Error != nil {
			return fmt.Errorf("failed to remove service: %w", deleteResult.Error)
		}
		if deleteResult.RowsAffected == 0 {
			return fmt.Errorf("service %q not found", args[0])
		}
		return nil
	},
}

func init() {
	serviceCmd.AddCommand(rmCmd)
}
