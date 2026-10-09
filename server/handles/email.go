package handles

import (
	"errors"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

type SendVerificationEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type VerifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

func isValidPkuEmail(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" {
		return false
	}

	matched, err := regexp.MatchString(`^\d+$`, parts[0])
	if err != nil || !matched {
		return false
	}

	switch parts[1] {
	case "stu.pku.edu.cn", "pku.edu.cn":
		return true
	default:
		return false
	}
}

func extractStuidFromEmail(email string) string {
	return strings.Split(email, "@")[0]
}

func SendVerificationEmail(c *gin.Context) {
	var req SendVerificationEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "invalid_request", err)
		return
	}

	// 验证邮箱是否为北大邮箱
	if !isValidPkuEmail(req.Email) {
		utils.RespondError(c, 400, "invalid_email_domain", errors.New("email must be a numeric student ID followed by @stu.pku.edu.cn or @pku.edu.cn"))
		return
	}

	code, err := utils.GenerateVerificationCode()
	if err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	if err := db.CreateEmailVerification(req.Email, code); err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	if err := utils.SendVerificationEmail(req.Email, code); err != nil {
		utils.RespondError(c, 500, "failed_to_send_email", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"message": "verification email sent successfully"})
}

func VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "invalid_request", err)
		return
	}

	if !isValidPkuEmail(req.Email) {
		utils.RespondError(c, 400, "invalid_email_domain", errors.New("email must be a numeric student ID followed by @stu.pku.edu.cn or @pku.edu.cn"))
		return
	}

	verification, err := db.GetEmailVerificationByEmail(req.Email)
	if err != nil {
		utils.RespondError(c, 400, "invalid_verification_code", errors.New("invalid or expired verification code"))
		return
	}

	if verification.Code != req.Code {
		utils.RespondError(c, 400, "invalid_verification_code", errors.New("invalid verification code"))
		return
	}

	if err := db.MarkEmailVerificationAsUsed(req.Email); err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	stuid := extractStuidFromEmail(req.Email)
	user, err := db.GetUserByStuid(stuid)
	if err != nil {
		user = &model.User{
			Username: stuid,
			Stuid:    stuid,
			Role:     model.GENERAL,
			Verified: true,
		}

		if err := db.CreateUser(user); err != nil {
			utils.RespondError(c, 500, "internal_server_error", errors.New("failed to create user: "+err.Error()))
			return
		}
	}

	token, err := utils.GenerateToken(user)
	if err != nil {
		utils.RespondError(c, 500, "internal_server_error", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"token": token, "username": user.Username, "userid": user.ID})
}
