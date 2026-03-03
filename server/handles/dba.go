package handles

import (
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

func CreateAll(c *gin.Context) {
	if err := db.CreateAll(); err != nil {
		utils.RespondError(c, 500, "ServerInternalError", err)
		return
	}
}

func ListTables(c *gin.Context) {
	tables, err := db.ListTables()
	if err != nil {
		utils.RespondError(c, 500, "ServerInternalError", err)
		return
	}
	utils.RespondSuccess(c, tables)
}
