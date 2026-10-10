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

func MarkNotificationRead(userID uint, id uint) error {
	return db.Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("read", true).Error
}

func MarkAllNotificationsRead(userID uint) error {
	return db.Model(&model.Notification{}).
		Where("user_id = ? AND read = ?", userID, false).
		Update("read", true).Error
}

// NotificationExists 判断是否已存在同类通知，用于点赞/关注通知去重
func NotificationExists(userID uint, notifType string, relatedUserID, relatedPostID, relatedCommentID uint) (bool, error) {
	query := db.Model(&model.Notification{}).
		Where("user_id = ? AND type = ? AND related_user_id = ?", userID, notifType, relatedUserID)
	if relatedPostID != 0 {
		query = query.Where("related_post_id = ?", relatedPostID)
	}
	if relatedCommentID != 0 {
		query = query.Where("related_comment_id = ?", relatedCommentID)
	}

	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

// GetUsernamesByIDs 批量查询用户名，供通知列表补充发起人昵称
func GetUsernamesByIDs(ids []uint) (map[uint]string, error) {
	result := make(map[uint]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	var users []model.User
	if err := db.Select("id", "username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		result[user.ID] = user.Username
	}
	return result, nil
}
