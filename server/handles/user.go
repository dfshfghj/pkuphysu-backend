package handles

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
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

	type CurrentUserResponse struct {
		model.User  `json:",inline"`
		HasPassword bool `json:"has_password"`
	}

	response := CurrentUserResponse{
		User:        *user,
		HasPassword: user.PwdHash != "",
	}

	utils.RespondSuccess(c, response)
}

func DeleteUser(c *gin.Context) {
	currentUser := c.MustGet("CurrentUser").(*model.User)

	err := db.DeleteUserById(currentUser.ID)
	if err != nil {
		utils.RespondError(c, 500, "failed_to_delete_user", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"message": "user_deleted_successfully"})
}

func UpdateUserInfo(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"omitempty,min=1,max=50"`
		Bio      string `json:"bio" binding:"omitempty,max=200"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "invalid_request", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	if req.Username != "" && req.Username != currentUser.Username {
		_, err := db.GetUserByName(req.Username)
		if err == nil {
			utils.RespondError(c, 409, "username_already_exists", nil)
			return
		}

		currentUser.Username = req.Username
	}

	if req.Bio != "" {
		currentUser.Bio = req.Bio
	}

	err := db.UpdateUser(currentUser)
	if err != nil {
		utils.RespondError(c, 500, "failed_to_update_user", err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"message":  "user_info_updated_successfully",
		"username": currentUser.Username,
		"bio":      currentUser.Bio,
	})
}

func UploadAvatar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		utils.RespondError(c, 400, "invalid_request", errors.New("avatar file is required"))
		return
	}

	if file.Size > 5*1024*1024 {
		utils.RespondError(c, 400, "file_too_large", errors.New("file size must be less than 5MB"))
		return
	}

	if !utils.IsValidImageType(file.Filename) {
		utils.RespondError(c, 400, "invalid_file_type", errors.New("only jpg, jpeg, png, gif, and webp files are allowed"))
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	avatarDir := "./data/avatar"
	if err := utils.EnsureDirExists(avatarDir); err != nil {
		utils.RespondError(c, 500, "failed_to_create_avatar_dir", err)
		return
	}

	filePath := filepath.Join(avatarDir, fmt.Sprintf("%d", currentUser.ID))

	src, err := file.Open()
	if err != nil {
		utils.RespondError(c, 500, "failed_to_read_file", err)
		return
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		utils.RespondError(c, 500, "failed_to_read_file_content", err)
		return
	}

	if err := utils.SaveFile(filePath, data); err != nil {
		utils.RespondError(c, 500, "failed_to_save_avatar", err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"message": "avatar_uploaded_successfully",
	})
}

func GetAvatar(c *gin.Context) {
	userID := c.Param("id")

	_, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "invalid_user_id", errors.New("user ID must be a valid number"))
		return
	}

	avatarDir := "./data/avatar"
	filePath := filepath.Join(avatarDir, userID)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		utils.RespondError(c, 404, "avatar_not_found", errors.New("avatar not found"))
		return
	}

	c.File(filePath)
}
