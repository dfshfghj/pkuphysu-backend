package handles

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/utils"
)

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password"`
}

func Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}
	user, err := db.GetUserByName(req.Username)
	if err != nil {
		utils.RespondError(c, http.StatusUnauthorized, "invalid_credentials", err)
	}

	if err := user.ValidatePassword(req.Password); err != nil {
		utils.RespondError(c, http.StatusUnauthorized, "invalid_credentials", err)
	} else {
		token, err := utils.GenerateToken(user)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "internal_server_error", err)
		} else {
			utils.RespondSuccess(c, gin.H{"token": token})
		}
	}
}