package db

import (
	"fmt"
	"strings"

	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/model"

	"github.com/sirupsen/logrus"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	db *gorm.DB
)

func InitDB() {
	gormConfig := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: config.Conf.Database.TablePrefix,
		},
	}

	datebase := config.Conf.Database

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		datebase.Host, datebase.User, datebase.Password, datebase.DBName, datebase.Port, datebase.SSLMode)

	logrus.Info("Initializing database connection...")
	var err error
	db, err = gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to database")
	}

	logrus.Info("Database connected successfully")

	if err := CreateAll(); err != nil {
		logrus.WithError(err).Fatal("Failed to create/migrate tables")
	}

	logrus.Info("Database tables created/migrated successfully")
}

func CreateAll() error {
	for _, model := range model.Models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}
	return nil
}

// ListTables returns a map of table information
func ListTables() (map[string]map[string]interface{}, error) {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return nil, err
	}
	existingTable := make(map[string]bool)
	for _, t := range tables {
		existingTable[t] = true
	}

	result := make(map[string]map[string]interface{})
	for _, model := range model.Models {
		statement := &gorm.Statement{DB: db}
		if err := statement.Parse(model); err != nil {
			continue
		}
		tableName := statement.Schema.Table

		info := map[string]interface{}{
			"exists": false,
			"rows":   0,
		}

		if existingTable[tableName] {
			info["exists"] = true
			var count int64
			if err := db.Table(tableName).Count(&count).Error; err != nil {
				count = -1 // or skip
			}
			info["rows"] = count
		}
		result[tableName] = info
	}
	return result, nil
}

func GetTableData(tableName string) (map[string]interface{}, error) {
	exists := db.Migrator().HasTable(tableName)
	if !exists {
		return nil, fmt.Errorf("table %s does not exist", tableName)
	}

	columnTypes, err := db.Migrator().ColumnTypes(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}

	var columnNames []string
	var columnTypesInfo []string

	primaryKeys, err := getPrimaryKeys(tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary keys: %w", err)
	}
	primaryKeyMap := make(map[string]bool)
	for _, pk := range primaryKeys {
		primaryKeyMap[pk] = true
	}

	for _, col := range columnTypes {
		columnNames = append(columnNames, col.Name())
		typeStr := col.DatabaseTypeName()
		nullable, ok := col.Nullable()
		notNullStr := ""
		if ok && !nullable {
			notNullStr = " NOT NULL "
		}

		pkStr := ""
		if primaryKeyMap[col.Name()] {
			pkStr = " PRIMARY KEY "
		}

		typeInfo := fmt.Sprintf("%s%s%s", typeStr, notNullStr, pkStr)
		columnTypesInfo = append(columnTypesInfo, strings.TrimSpace(typeInfo))
	}

	var results []map[string]interface{}
	if err := db.Table(tableName).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to query table: %w", err)
	}

	return map[string]interface{}{
		"table":   tableName,
		"columns": columnNames,
		"types":   columnTypesInfo,
		"count":   len(results),
		"data":    results,
	}, nil
}

func DeleteTableRecords(tableName string, data interface{}) (int, error) {
	exists := db.Migrator().HasTable(tableName)
	if !exists {
		return 0, fmt.Errorf("table %s does not exist", tableName)
	}

	primaryKeys, err := getPrimaryKeys(tableName)
	if err != nil {
		return 0, fmt.Errorf("failed to get primary keys: %w", err)
	}
	if len(primaryKeys) == 0 {
		return 0, fmt.Errorf("table has no primary key")
	}

	deletedCount := 0

	if dataStr, ok := data.(string); ok && dataStr == "all" {
		if err := db.Exec(fmt.Sprintf(`TRUNCATE TABLE "%s" RESTART IDENTITY CASCADE`, tableName)).Error; err != nil {
			return 0, fmt.Errorf("failed to truncate table: %w", err)
		}
		return 0, nil
	}

	dataArray, ok := data.([]interface{})
	if !ok || len(dataArray) == 0 {
		return 0, fmt.Errorf("no data provided for deletion")
	}

	for _, record := range dataArray {
		recordMap, ok := record.(map[string]interface{})
		if !ok {
			continue
		}

		conditions := make(map[string]interface{})
		for _, pk := range primaryKeys {
			val, exists := recordMap[pk]
			if !exists {
				return 0, fmt.Errorf("missing primary key value: %s", pk)
			}
			conditions[pk] = val
		}

		result := db.Table(tableName).Where(conditions).Delete(nil)
		if result.Error != nil {
			return deletedCount, fmt.Errorf("failed to delete record: %w", result.Error)
		}
		deletedCount += int(result.RowsAffected)
	}

	return deletedCount, nil
}

func UpsertTableRecords(tableName string, records []map[string]interface{}) (int, int, error) {
	exists := db.Migrator().HasTable(tableName)
	if !exists {
		return 0, 0, fmt.Errorf("table %s does not exist", tableName)
	}

	columnTypes, err := db.Migrator().ColumnTypes(tableName)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get column types: %w", err)
	}

	columnNames := make(map[string]bool)
	for _, col := range columnTypes {
		columnNames[col.Name()] = true
	}

	primaryKeys, err := getPrimaryKeys(tableName)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get primary keys: %w", err)
	}
	if len(primaryKeys) == 0 {
		return 0, 0, fmt.Errorf("table has no primary key")
	}

	insertedCount := 0
	updatedCount := 0

	for _, record := range records {
		filteredRecord := make(map[string]interface{})
		for k, v := range record {
			if columnNames[k] {
				filteredRecord[k] = v
			}
		}

		if len(filteredRecord) == 0 {
			return 0, 0, fmt.Errorf("invalid column in data")
		}

		conditions := make(map[string]interface{})
		for _, pk := range primaryKeys {
			val, exists := filteredRecord[pk]
			if val == nil || !exists {
				return 0, 0, fmt.Errorf("primary key cannot be null: %s", pk)
			}
			conditions[pk] = val
		}

		var count int64
		if err := db.Table(tableName).Where(conditions).Count(&count).Error; err != nil {
			return 0, 0, fmt.Errorf("failed to check record existence: %w", err)
		}

		if count > 0 {
			updateData := make(map[string]interface{})
			for k, v := range filteredRecord {
				isPrimaryKey := false
				for _, pk := range primaryKeys {
					if k == pk {
						isPrimaryKey = true
						break
					}
				}
				if !isPrimaryKey {
					updateData[k] = v
				}
			}

			if len(updateData) > 0 {
				result := db.Table(tableName).Where(conditions).Updates(updateData)
				if result.Error != nil {
					return insertedCount, updatedCount, fmt.Errorf("update failed: %w", result.Error)
				}
				updatedCount += int(result.RowsAffected)
			}
		} else {
			result := db.Table(tableName).Create(filteredRecord)
			if result.Error != nil {
				return insertedCount, updatedCount, fmt.Errorf("insert failed: %w", result.Error)
			}
			insertedCount += int(result.RowsAffected)
		}
	}

	return insertedCount, updatedCount, nil
}

func getPrimaryKeys(tableName string) ([]string, error) {
	query := `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = ?::regclass
		AND i.indisprimary
	`

	rows, err := db.Raw(query, tableName).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var primaryKeys []string
	for rows.Next() {
		var pk string
		if err := rows.Scan(&pk); err != nil {
			return nil, err
		}
		primaryKeys = append(primaryKeys, pk)
	}

	return primaryKeys, nil
}

func CheckMigration() ([]string, error) {
	var diffs []string

	for _, m := range model.Models {
		statement := &gorm.Statement{DB: db}
		if err := statement.Parse(m); err != nil {
			continue
		}

		tableName := statement.Schema.Table
		exists := db.Migrator().HasTable(tableName)

		if !exists {
			diffs = append(diffs, fmt.Sprintf("Table '%s' does not exist", tableName))
			continue
		}

		columnTypes, err := db.Migrator().ColumnTypes(tableName)
		if err != nil {
			diffs = append(diffs, fmt.Sprintf("Cannot inspect table '%s': %v", tableName, err))
			continue
		}

		existingColumns := make(map[string]string)
		for _, col := range columnTypes {
			sqlType := col.DatabaseTypeName()
			existingColumns[col.Name()] = sqlType
		}

		for _, field := range statement.Schema.Fields {
			if field.FieldType.Kind() == 35 { // skip embedded struct (reflect.Interface)
				continue
			}

			colName := field.DBName
			expectedType := field.DataType

			if existingType, exists := existingColumns[colName]; !exists {
				diffs = append(diffs, fmt.Sprintf("Column '%s' missing in table '%s'", colName, tableName))
			} else {
				if !typeMatch(expectedType, existingType) {
					diffs = append(diffs, fmt.Sprintf("Column '%s' type mismatch: expected %v, got %s",
						colName, expectedType, existingType))
				}
			}
		}
	}

	if len(diffs) == 0 {
		diffs = append(diffs, "No migration needed - database schema is up to date")
	}

	return diffs, nil
}

func ExecuteMigration() error {
	logrus.Info("Starting database migration...")

	for _, m := range model.Models {
		if err := db.AutoMigrate(m); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", m, err)
		}
		logrus.Infof("Migrated table: %T", m)
	}

	logrus.Info("Database migration completed successfully")
	return nil
}

func typeMatch(expected interface{}, actual string) bool {
	typeStr := fmt.Sprintf("%v", expected)

	typeMap := map[string][]string{
		"int":       {"INTEGER", "INT4", "SERIAL"},
		"int64":     {"BIGINT", "INT8", "BIGSERIAL"},
		"string":    {"VARCHAR", "TEXT", "CHAR"},
		"time.Time": {"TIMESTAMP", "TIMESTAMPTZ", "DATE"},
		"bool":      {"BOOLEAN", "BOOL"},
		"float64":   {"DOUBLE PRECISION", "FLOAT8", "REAL"},
	}

	for goType, sqlTypes := range typeMap {
		if contains(typeStr, goType) {
			for _, sqlType := range sqlTypes {
				if actual == sqlType {
					return true
				}
			}
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr))
}
