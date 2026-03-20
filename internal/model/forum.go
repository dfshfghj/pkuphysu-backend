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
	UserID      uint           `gorm:"constraint:OnDelete:CASCADE;"`
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
	PostID      uint           `gorm:"constraint:OnDelete:CASCADE;"`
	UserID      uint           `gorm:"constraint:OnDelete:CASCADE;"`
	User        *User
	QuoteID     *uint         `gorm:"constraint:OnDelete:SET NULL;"`
	Quote       *ForumComment `gorm:"foreignKey:QuoteID"`
}

type ForumFollow struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"constraint:OnDelete:CASCADE;"`
	PostID    uint `gorm:"constraint:OnDelete:CASCADE;"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
}

type ForumLike struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"constraint:OnDelete:CASCADE;"`
	PostID    uint `gorm:"constraint:OnDelete:CASCADE;"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
}

type CommentLike struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"constraint:OnDelete:CASCADE;"`
	CommentID uint `gorm:"constraint:OnDelete:CASCADE;"`
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
