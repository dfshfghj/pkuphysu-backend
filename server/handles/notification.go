package handles

import (
	"strconv"

	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

// GetNotifications 获取当前用户的通知列表（页面刷新时调用）
func GetNotifications(c *gin.Context) {
	user := c.MustGet("CurrentUser").(*model.User)

	// 获取最近50条通知
	notifications, err := db.GetNotificationsByUserID(user.ID, 50)
	if err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	// 获取未读数量
	count, err := db.GetUnreadNotificationCount(user.ID)
	if err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	// 批量补充发起通知的用户名，供前端拼装人性化文案
	relatedUserIDs := make([]uint, 0, len(notifications))
	seen := make(map[uint]bool, len(notifications))
	for _, notification := range notifications {
		if notification.RelatedUserID != 0 && !seen[notification.RelatedUserID] {
			seen[notification.RelatedUserID] = true
			relatedUserIDs = append(relatedUserIDs, notification.RelatedUserID)
		}
	}
	usernames, err := db.GetUsernamesByIDs(relatedUserIDs)
	if err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	items := make([]map[string]interface{}, len(notifications))
	for i, notification := range notifications {
		items[i] = map[string]interface{}{
			"id":                 notification.ID,
			"type":               notification.Type,
			"title":              notification.Title,
			"content":            notification.Content,
			"read":               notification.Read,
			"created_at":         notification.CreatedAt,
			"related_user_id":    notification.RelatedUserID,
			"related_username":   usernames[notification.RelatedUserID],
			"related_post_id":    notification.RelatedPostID,
			"related_comment_id": notification.RelatedCommentID,
		}
	}

	utils.RespondSuccess(c, gin.H{
		"notifications": items,
		"unread_count":  count,
	})
}

// GetUnreadNotificationCount 只返回未读数量，供侧栏红点轮询
func GetUnreadNotificationCount(c *gin.Context) {
	user := c.MustGet("CurrentUser").(*model.User)

	count, err := db.GetUnreadNotificationCount(user.ID)
	if err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"unread_count": count})
}

// MarkNotificationRead 将单条通知标记为已读
func MarkNotificationRead(c *gin.Context) {
	user := c.MustGet("CurrentUser").(*model.User)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "invalid_notification_id", err)
		return
	}

	if err := db.MarkNotificationRead(user.ID, uint(id)); err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	count, err := db.GetUnreadNotificationCount(user.ID)
	if err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"unread_count": count})
}

// MarkAllNotificationsRead 将当前用户的全部通知标记为已读
func MarkAllNotificationsRead(c *gin.Context) {
	user := c.MustGet("CurrentUser").(*model.User)

	if err := db.MarkAllNotificationsRead(user.ID); err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"unread_count": 0})
}
