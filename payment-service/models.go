package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

func initDB() {
	dsn := "host=localhost user=postgres password=postgres dbname=payment_system port=5432 sslmode=disable TimeZone=Asia/Almaty"
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to PostgreSQL")

	err = db.AutoMigrate(&Balance{}, &Transaction{})
	if err != nil {
		log.Fatal("Migration failed")
	}
	fmt.Println("Migrations applied")
}

type Balance struct {
	UserID    uint    `gorm:"primaryKey"`
	Balance   float64 `gorm:"not null;default:0"`
	Version   int     `gorm:"not null;default:1"`
	UpdatedAt time.Time
}

type Transaction struct {
	ID              uint `gorm:"primaryKey"`
	SenderID        uint `gorm:"not null"`
	RecipientID     *uint
	Amount          float64 `gorm:"not null"`
	Status          string  `gorm:"not null"`
	TransactionType string  `gorm:"not null"`
	Description     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
