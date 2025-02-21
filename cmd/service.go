package cmd

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

const DefaultSize = 64

type Service struct {
	Token       string `gorm:"primaryKey"`
	ServiceName string `gorm:"unique"`
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "managing services registered to database",
}

func init() {
	rootCmd.AddCommand(serviceCmd)
}

func migrate() {
	db, err := gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("%v", err))
	}
	db.AutoMigrate(&Service{})
}
