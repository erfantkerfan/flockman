package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var listCmd = &cobra.Command{
	Use:   "ls",
	Short: "list all services",
	Run: func(cmd *cobra.Command, args []string) {
		migrate()
		db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
		if err != nil {
			panic(fmt.Errorf("%v", err))
		}
		var result []Service
		_ = db.Find(&result)
		tw := tabwriter.NewWriter(os.Stdout, 64, 0, 2, ' ', tabwriter.TabIndent)
		fmt.Fprintln(tw, "SERVICE_NAME\tTOKEN\t")
		for _, v := range result {
			fmt.Fprintf(tw, "%v\t%v\n", v.ServiceName, v.Token)
		}
		tw.Flush()
	},
}

func init() {
	serviceCmd.AddCommand(listCmd)
}
