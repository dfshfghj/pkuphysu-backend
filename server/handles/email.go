package handles

import (
	"errors"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SendVerificationEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type VerifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

func isValidPkuStudentEmail(email string) bool {
	return strings.HasSuffix(email, "@stu.pku.edu.cn")
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
	if !isValidPkuStudentEmail(req.Email) {
		utils.RespondError(c, 400, "invalid_email_domain", errors.New("email must be @stu.pku.edu.cn domain"))
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

	// 验证邮箱是否为北大邮箱
	if !isValidPkuStudentEmail(req.Email) {
		utils.RespondError(c, 400, "invalid_email_domain", errors.New("email must be @stu.pku.edu.cn domain"))
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
	_, err = db.GetUserByName(stuid)
	if err == nil {
		utils.RespondSuccess(c, gin.H{
			"message":  "email verified successfully",
			"password": "", // 不返回现有用户的密码
		})
		return
	}

	user := &model.User{
		Username: stuid,
		Stuid:    stuid,
		Stuname:  stuid,
		Role:     model.GUEST,
		Verified: true,
	}

	// 生成随机密码
	randomPassword, err := utils.GenerateRandomString(16)
	if err != nil {
		utils.RespondError(c, 500, "internal_server_error", errors.New("failed to generate random password"))
		return
	}
	user.SetPassword(randomPassword)

	if err := db.CreateUser(user); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "duplicate key") {
			_, getUserErr := db.GetUserByName(stuid)
			if getUserErr == nil {
				utils.RespondSuccess(c, gin.H{
					"message":  "email verified successfully",
					"password": "",
				})
				return
			}
		}
		utils.RespondError(c, 500, "internal_server_error", errors.New("failed to create user: "+err.Error()))
		return
	}

	utils.RespondSuccess(c, gin.H{
		"message":  "email verified successfully",
		"password": randomPassword,
	})
}
