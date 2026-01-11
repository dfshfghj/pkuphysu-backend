package db

import (
	"fmt"

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
	db, _ = gorm.Open(postgres.Open(dsn), gormConfig)
	db.AutoMigrate(&model.User{})
}