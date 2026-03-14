package handles

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"
)

type LoginReq struct {
	Username string `json:"username" binding:"required,min=1,max=50"`
	Password string `json:"password"`
}

type ChangePasswordReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

func Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	var user *model.User
	var err error

	if strings.Contains(req.Username, "@") {
		if !isValidPkuStudentEmail(req.Username) {
			utils.RespondError(c, http.StatusBadRequest, "invalid_email_format",
				fmt.Errorf("邮箱必须是@stu.pku.edu.cn域名，且前缀必须是学号"))
			return
		}

		stuid := extractStuidFromEmail(req.Username)
		user, err = db.GetUserByStuid(stuid)
	} else {
		user, err = db.GetUserByName(req.Username)
	}

	if err != nil {
		utils.RespondError(c, http.StatusUnauthorized, "invalid_credentials", err)
		return
	}

	if err := user.ValidatePassword(req.Password); err != nil {
		utils.RespondError(c, http.StatusUnauthorized, "invalid_credentials", err)
	} else {
		token, err := utils.GenerateToken(user)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "internal_server_error", err)
		} else {
			utils.RespondSuccess(c, gin.H{"token": token, "username": user.Username, "userid": user.ID})
		}
	}
}

func ChangePassword(c *gin.Context) {
	var req ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	user := c.MustGet("CurrentUser").(*model.User)
	if user.PwdHash == "" && user.Salt == "" {
	} else {
		if err := user.ValidatePassword(req.OldPassword); err != nil {
			utils.RespondError(c, http.StatusUnauthorized, "invalid_current_password", err)
			return
		}
	}

	user.SetPassword(req.NewPassword)

	if err := db.UpdateUser(user); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "failed_to_update_password", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"message": "password_updated_successfully"})
}
