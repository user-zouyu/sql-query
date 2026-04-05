package db

import "gorm.io/gorm"

// TableInfo represents a row from INFORMATION_SCHEMA.TABLES.
type TableInfo struct {
	TableName    string `json:"table_name" gorm:"column:table_name"`
	TableComment string `json:"table_comment" gorm:"column:table_comment"`
}

// GetTables lists all base tables in the current database.
func GetTables(db *gorm.DB) ([]TableInfo, error) {
	var tables []TableInfo
	err := db.Raw(`
		SELECT TABLE_NAME AS table_name,
		       COALESCE(TABLE_COMMENT, '') AS table_comment
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME
	`).Scan(&tables).Error
	if err != nil {
		return nil, err
	}
	for i := range tables {
		tables[i].TableComment = FixUTF8(tables[i].TableComment)
	}
	return tables, nil
}
