package handles

import (
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

	utils.RespondSuccess(c, gin.H{
		"notifications": notifications,
		"unread_count":  count,
	})
}
