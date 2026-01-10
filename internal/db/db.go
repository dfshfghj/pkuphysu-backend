package db

import (
	"fmt"

	"pkuphysu-backend/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	dB *gorm.DB
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
	dB, _ = gorm.Open(postgres.Open(dsn), gormConfig)
}