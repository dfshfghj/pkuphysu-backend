package model

import (
	"time"

	"gorm.io/gorm"
)

type ForumPost struct {
	ID        uint `gorm:"primaryKey"`
	Content   string
	Reply     int
	Follownum int // 关注数量（原Likenum字段）
	Likenum   int // 点赞数量（新增字段）
	Type      int
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
	UserID    uint
	User      *User
	Comments  []ForumComment `gorm:"foreignKey:PostID"`
	Tags      []ForumTag     `gorm:"many2many:forum_post_tags;"`
}

type ForumComment struct {
	ID        uint `gorm:"primaryKey"`
	Content   string
	Likenum   int // 评论点赞数量
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
	PostID    uint
	UserID    uint
	User      *User
	QuoteID   *uint
	Quote     *ForumComment `gorm:"foreignKey:QuoteID"`
}

type ForumFollow struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint
	PostID    uint
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
}

// ForumLike 表示用户对帖子的点赞关系
type ForumLike struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint
	PostID    uint
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
}

// CommentLike 表示用户对评论的点赞关系
type CommentLike struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint
	CommentID uint
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
}

// ForumTag 表示论坛标签
type ForumTag struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"uniqueIndex"`
	IsDefault bool   `gorm:"default:false"` // 标识是否为系统默认标签
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `json:"-"`
}

// ForumPostTag 是帖子和标签的关联表
type ForumPostTag struct {
	PostID uint `gorm:"primaryKey"`
	TagID  uint `gorm:"primaryKey"`
}
