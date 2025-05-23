package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB
var tokenBlacklist = make(map[string]time.Time)

func initDB() {
	dsn := "host=localhost user=postgres password=postgres dbname=payment_system port=5432 sslmode=disable TimeZone=Asia/Almaty"
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to PostgreSQL")

	err = db.AutoMigrate(&User{}, &RefreshToken{})
	if err != nil {
		log.Fatal("Migration failed")
	}
	fmt.Println("Migrations applied")
}

type User struct {
	ID           uint   `gorm:"primaryKey"`
	PhoneNumber  string `gorm:"unique;not null"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type RefreshToken struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"not null"`
	Token     string `gorm:"unique;not null"`
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}
