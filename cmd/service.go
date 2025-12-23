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

// db is a singleton database connection
var db *gorm.DB

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "managing services registered to database",
}

func init() {
	rootCmd.AddCommand(serviceCmd)
}

// initDB initializes the database connection and runs migrations
func initDB() error {
	var err error
	db, err = gorm.Open(sqlite.Open(DatabaseFile), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.AutoMigrate(&Service{}); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	return nil
}

// getDB returns the database connection, initializing it if needed
func getDB() (*gorm.DB, error) {
	if db == nil {
		if err := initDB(); err != nil {
			return nil, err
		}
	}
	return db, nil
}
