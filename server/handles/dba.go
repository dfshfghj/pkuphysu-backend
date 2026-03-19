package handles

import (
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

func CreateAll(c *gin.Context) {
	if err := db.CreateAll(); err != nil {
		utils.RespondError(c, 500, "CreateTablesFailed", err)
		return
	}
	utils.RespondSuccess(c, gin.H{"message": "tables_created_successfully"})
}

func ListTables(c *gin.Context) {
	tables, err := db.ListTables()
	if err != nil {
		utils.RespondError(c, 500, "ListTablesFailed", err)
		return
	}
	utils.RespondSuccess(c, tables)
}

func GetTableData(c *gin.Context) {
	tableName := c.Param("table")

	data, err := db.GetTableData(tableName)
	if err != nil {
		utils.RespondError(c, 500, "QueryTableFailed", err)
		return
	}

	utils.RespondSuccess(c, data)
}

func DeleteTableRecords(c *gin.Context) {
	tableName := c.Param("table")

	var req struct {
		Data interface{} `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidRequest", err)
		return
	}

	deleted, err := db.DeleteTableRecords(tableName, req.Data)
	if err != nil {
		utils.RespondError(c, 500, "DeleteFailed", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"deleted": deleted})
}

func UpsertTableRecords(c *gin.Context) {
	tableName := c.Param("table")

	var req struct {
		Data []map[string]interface{} `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidRequest", err)
		return
	}

	inserted, updated, err := db.UpsertTableRecords(tableName, req.Data)
	if err != nil {
		utils.RespondError(c, 500, "UpsertFailed", err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"inserted": inserted,
		"updated":  updated,
	})
}

func CheckMigration(c *gin.Context) {
	diffs, err := db.CheckMigration()
	if err != nil {
		utils.RespondError(c, 500, "MigrationCheckFailed", err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"migration": diffs,
		"count":     len(diffs),
	})
}

func ExecuteMigration(c *gin.Context) {
	if err := db.ExecuteMigration(); err != nil {
		utils.RespondError(c, 500, "MigrationFailed", err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"message": "Database migration completed successfully",
	})
}
