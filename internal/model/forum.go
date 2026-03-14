package model

import (
	"time"

	"gorm.io/gorm"
)

type ForumPost struct {
	ID          uint `gorm:"primaryKey"`
	Content     string
	ContentHTML string
	ContentText string
	Reply       int
	Follownum   int
	Likenum     int
	Type        int
	CreatedAt   time.Time
	DeletedAt   gorm.DeletedAt `json:"-"`
	UserID      uint
	User        *User
	Comments    []ForumComment `gorm:"foreignKey:PostID"`
	Tags        []ForumTag     `gorm:"many2many:forum_post_tags;"`
}

type ForumComment struct {
	ID          uint `gorm:"primaryKey"`
	Content     string
	ContentHTML string
	ContentText string
	Likenum     int
	CreatedAt   time.Time
	DeletedAt   gorm.DeletedAt `json:"-"`
	PostID      uint
	UserID      uint
	User        *User
	QuoteID     *uint
	Quote       *ForumComment `gorm:"foreignKey:QuoteID"`
}

type ForumFollow struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint
	PostID    uint
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
}

type ForumLike struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint
	PostID    uint
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
}

type CommentLike struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint
	CommentID uint
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
}

type ForumTag struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"uniqueIndex"`
	IsDefault bool   `gorm:"default:false"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
}

type ForumPostTag struct {
	PostID uint `gorm:"primaryKey"`
	TagID  uint `gorm:"primaryKey"`
}
