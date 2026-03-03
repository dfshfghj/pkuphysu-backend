package db

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	db *gorm.DB
)

func InitDB() {
	gormConfig := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: config.Conf.Database.TablePrefix,
		},
	}

	datebase := config.Conf.Database

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		datebase.Host, datebase.User, datebase.Password, datebase.DBName, datebase.Port, datebase.SSLMode)

	logrus.Info("Initializing database connection...")
	var err error
	db, err = gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to database")
	}

	logrus.Info("Database connected successfully")

	if err := CreateAll(); err != nil {
		logrus.WithError(err).Fatal("Failed to create/migrate tables")
	}

	logrus.Info("Database tables created/migrated successfully")
}

func CreateAll() error {
	for _, model := range model.Models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}
	return nil
}

func ListTables() (map[string]map[string]interface{}, error) {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return nil, err
	}
	existingTable := make(map[string]bool)
	for _, t := range tables {
		existingTable[t] = true
	}

	result := make(map[string]map[string]interface{})
	for _, model := range model.Models {
		statement := &gorm.Statement{DB: db}
		if err := statement.Parse(model); err != nil {
			continue
		}
		tableName := statement.Schema.Table

		info := map[string]interface{}{
			"exists": false,
			"rows":   0,
		}

		if existingTable[tableName] {
			info["exists"] = true
			var count int64
			if err := db.Table(tableName).Count(&count).Error; err != nil {
				count = -1 // or skip
			}
			info["rows"] = count
		}
		result[tableName] = info
	}
	return result, nil
}

func GetTableData() {

}
