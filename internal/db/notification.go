package db

import (
	"pkuphysu-backend/internal/model"
)

func CreateNotification(notification *model.Notification) error {
	return db.Create(notification).Error
}

func GetNotificationsByUserID(userID uint, limit int) ([]model.Notification, error) {
	var notifications []model.Notification
	err := db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&notifications).Error
	return notifications, err
}

func GetUnreadNotificationCount(userID uint) (int64, error) {
	var count int64
	err := db.Model(&model.Notification{}).
		Where("user_id = ? AND read = ?", userID, false).
		Count(&count).Error
	return count, err
}
