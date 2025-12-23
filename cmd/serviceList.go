package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "ls",
	Short: "list all services",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := getDB()
		if err != nil {
			return err
		}
		var result []Service
		if err := db.Find(&result).Error; err != nil {
			return fmt.Errorf("failed to list services: %w", err)
		}
		tw := tabwriter.NewWriter(os.Stdout, 64, 0, 2, ' ', tabwriter.TabIndent)
		fmt.Fprintln(tw, "SERVICE_NAME\tTOKEN\t")
		for _, v := range result {
			fmt.Fprintf(tw, "%v\t%v\n", v.ServiceName, v.Token)
		}
		tw.Flush()
		return nil
	},
}

func init() {
	serviceCmd.AddCommand(listCmd)
}
