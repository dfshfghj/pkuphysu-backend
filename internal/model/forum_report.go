package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	ForumReportTargetPost    = "post"
	ForumReportTargetComment = "comment"

	ForumReportStatusPending  = "pending"
	ForumReportStatusResolved = "resolved"
)

type ForumReport struct {
	ID         uint   `gorm:"primaryKey"`
	TargetType string `gorm:"type:varchar(16);index"`
	TargetID   uint   `gorm:"index"`
	Reason     string `gorm:"type:varchar(128)"`
	Detail     string
	Status     string `gorm:"type:varchar(16);index;default:'pending'"`
	CreatedAt  time.Time
	ReviewedAt *time.Time
	DeletedAt  gorm.DeletedAt `json:"-"`

	ReporterID uint `gorm:"index;constraint:OnDelete:CASCADE;"`
	Reporter   *User
	ReviewedBy uint

	PostID    *uint `gorm:"index;constraint:OnDelete:CASCADE;"`
	CommentID *uint `gorm:"index;constraint:OnDelete:CASCADE;"`
}

func IsValidForumReportTarget(targetType string) bool {
	return targetType == ForumReportTargetPost || targetType == ForumReportTargetComment
}
