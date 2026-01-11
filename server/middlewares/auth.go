package middlewares

import (
	"errors"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

func Auth() func(c *gin.Context) {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			utils.RespondError(c, 401, "Unauthorized", errors.New("unauthorized"))
			c.Abort()
			return
		}
		userClaims, err := utils.ParseToken(token)
		if err != nil {
			utils.RespondError(c, 401, "invalid token", err)
			c.Abort()
			return
		}
		user, err := db.GetUserByName(userClaims.Username)
		if err != nil {
			utils.RespondError(c, 401, "invalid token", err)
			c.Abort()
			return
		}
		c.Set("CurrentUser", user)

		c.Next()
	}
}