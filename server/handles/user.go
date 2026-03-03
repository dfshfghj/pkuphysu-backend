package handles

import (
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

func CreateUser(c *gin.Context) {
	var req struct {
		model.User    `json:",inline"`
		PwdStaticHash string `json:"password" binding:"required"`
	}
	if err := c.ShouldBind(&req); err != nil {
		utils.RespondError(c, 400, "invalid_request", err)
		return
	}
	req.User.SetPassword(req.PwdStaticHash)
	if err := db.CreateUser(&req.User); err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
	} else {
		utils.RespondSuccess(c, gin.H{"user": req.User})
	}
}

func CurrentUser(c *gin.Context) {
	user := c.MustGet("CurrentUser").(*model.User)
	utils.RespondSuccess(c, user)
}
