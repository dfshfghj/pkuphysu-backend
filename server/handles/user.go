package handles

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"
	"strconv"
	"strings"

	"image/color"
	_ "image/jpeg"
	"image/png"
	_ "image/png"

	"github.com/gin-gonic/gin"
	"github.com/nfnt/resize"
	"github.com/pkg/errors"
	"github.com/rrivera/identicon"
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

	req.User.Verified = true

	if req.User.Role == 0 {
		req.User.Role = model.GENERAL
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
		Username string `json:"username" binding:"omitempty,alphanumunicode,min=1,max=50"`
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

var avatarSizeBuckets = []int{32, 48, 64, 96, 128, 192, 256}

func avatarBucket(size int) int {
	if size <= 0 {
		return 200
	}
	for _, bucket := range avatarSizeBuckets {
		if size <= bucket {
			return bucket
		}
	}
	return avatarSizeBuckets[len(avatarSizeBuckets)-1]
}

func GetAvatar(c *gin.Context) {
	userID := c.Param("id")

	_, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "invalid_user_id", errors.New("user ID must be a valid number"))
		return
	}

	size := 0
	if parsed, err := strconv.Atoi(c.Query("size")); err == nil {
		size = parsed
	}
	bucket := avatarBucket(size)

	avatarDir := "./data/avatar"
	filePath := filepath.Join(avatarDir, userID)
	imgFile, err := os.Open(filePath)

	if err != nil {
		ig, _ := identicon.New(
			"github", // Namespace
			5,        // Number of blocks (Size)
			3,        // Density
			identicon.SetBackgroundColorFunction(func([]byte, color.Color) color.Color {
				return color.Transparent
			}),
		)
		ii, _ := ig.Draw(userID)
		c.Header("Content-Type", "image/png")
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		_ = ii.Png(300, c.Writer)
		return
	}

	defer imgFile.Close()

	stat, err := imgFile.Stat()
	if err != nil {
		utils.RespondError(c, 500, "failed_to_stat_avatar", err)
		return
	}

	etag := fmt.Sprintf(`"%s-%d-%d-%d"`, userID, bucket, stat.Size(), stat.ModTime().UnixNano())
	c.Header("ETag", etag)
	c.Header("Cache-Control", "no-cache")

	if match := c.GetHeader("If-None-Match"); match == "*" || strings.Contains(match, etag) {
		c.Status(304)
		return
	}

	src, _, err := image.Decode(imgFile)
	if err != nil {
		utils.RespondError(c, 500, "failed_to_decode_avatar", err)
		return
	}
	resizedImg := resize.Resize(uint(bucket), uint(bucket), src, resize.Lanczos3)
	c.Header("Content-Type", "image/png")
	if err := png.Encode(c.Writer, resizedImg); err != nil {
		utils.RespondError(c, 500, "failed_to_encode_avatar", err)
		return
	}
}

func ListUsers(c *gin.Context) {
	users, err := db.GetUsers()
	if err != nil {
		utils.RespondError(c, 500, "failed_to_get_users", err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"users": users,
		"count": len(users),
	})
}

const (
	defaultUserSearchLimit = 8
	maxUserSearchLimit     = 20
)

func SearchUsers(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))

	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultUserSearchLimit)))
	if err != nil || limit <= 0 {
		limit = defaultUserSearchLimit
	}
	if limit > maxUserSearchLimit {
		limit = maxUserSearchLimit
	}

	users, err := db.SearchUsersByUsername(keyword, limit)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	result := make([]map[string]interface{}, len(users))
	for i, user := range users {
		result[i] = map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"verified": user.Verified,
		}
	}

	utils.RespondSuccess(c, result)
}

func ListAdmins(c *gin.Context) {
	admins, err := db.GetAllAdmins()
	if err != nil {
		utils.RespondError(c, 500, "failed_to_get_admins", err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"admins": admins,
		"count":  len(admins),
	})
}
