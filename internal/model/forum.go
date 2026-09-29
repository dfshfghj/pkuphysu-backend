package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	ForumContentStatusPending      = "pending"
	ForumContentStatusApproved     = "approved"
	ForumContentStatusManualReview = "manual_review"
	ForumContentStatusRejected     = "rejected"
)

type ForumPost struct {
	ID           uint `gorm:"primaryKey"`
	Content      string
	ContentHTML  string
	ContentText  string
	Status       string `gorm:"type:varchar(32);index;default:'pending'"`
	Reply        int
	Follownum    int
	Likenum      int
	Type         int
	EditCount    int
	LastEditedAt *time.Time
	CreatedAt    time.Time
	DeletedAt    gorm.DeletedAt `json:"-"`
	UserID       uint           `gorm:"constraint:OnDelete:CASCADE;"`
	User         *User
	Comments     []ForumComment `gorm:"foreignKey:PostID"`
	Tags         []ForumTag     `gorm:"many2many:forum_post_tags;"`
}

// ForumPostVersion 保存帖子被修改前的历史内容快照
type ForumPostVersion struct {
	ID          uint `gorm:"primaryKey"`
	PostID      uint `gorm:"index;constraint:OnDelete:CASCADE;"`
	Version     int
	Content     string
	ContentHTML string
	ContentText string
	CreatedAt   time.Time
}

type ForumComment struct {
	ID          uint `gorm:"primaryKey"`
	Content     string
	ContentHTML string
	ContentText string
	Status      string `gorm:"type:varchar(32);index;default:'pending'"`
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
	PostID uint `gorm:"column:forum_post_id;primaryKey"`
	TagID  uint `gorm:"column:forum_tag_id;primaryKey"`
}
