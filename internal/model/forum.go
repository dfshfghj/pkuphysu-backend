package model

import (
	"time"

	"gorm.io/gorm"
)

type ForumPost struct {
	ID        uint `gorm:"primaryKey"`
	Content   string
	Reply     int
	Likenum   int
	Type      int
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
	UserID    uint
	User      *User
	Comments  []ForumComment `gorm:"foreignKey:PostID"`
}

type ForumComment struct {
	ID        uint `gorm:"primaryKey"`
	Content   string
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
	PostID    uint
	UserID    uint
	User      *User
	QuoteID   *uint
	Quote     *ForumComment `gorm:"foreignKey:QuoteID"`
}
