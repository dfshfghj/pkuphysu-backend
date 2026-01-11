package handles

import (
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

func CreateUser(c *gin.Context) {
	var req struct {
		User model.User    `json:",inline"`
		PwdStaticHash string `json:"password" binding:"required"`
	}
	if err := c.ShouldBind(&req); err != nil {
		utils.RespondError(c, 400, "invalid_request", err)
		return
	}
	user := req.User
	user.SetPassword(req.PwdStaticHash)
	
	if err := db.CreateUser(&user); err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
	} else {
		utils.RespondSuccess(c, gin.H{"user": user})
	}
}
