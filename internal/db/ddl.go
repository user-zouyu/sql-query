package db

import (
	"fmt"

	"gorm.io/gorm"
)

// ColumnInfo represents a column definition from INFORMATION_SCHEMA.COLUMNS.
type ColumnInfo struct {
	Name     string  `json:"name" gorm:"column:COLUMN_NAME"`
	Type     string  `json:"type" gorm:"column:COLUMN_TYPE"`
	Nullable string  `json:"nullable" gorm:"column:IS_NULLABLE"`
	Default  *string `json:"default" gorm:"column:COLUMN_DEFAULT"`
	Comment  string  `json:"comment" gorm:"column:COLUMN_COMMENT"`
}

// IndexInfo represents an index from INFORMATION_SCHEMA.STATISTICS.
type IndexInfo struct {
	Name      string `json:"name" gorm:"column:INDEX_NAME"`
	Columns   string `json:"columns" gorm:"column:COLUMNS"`
	IsUnique  bool   `json:"is_unique" gorm:"column:IS_UNIQUE"`
	IsPrimary bool   `json:"is_primary" gorm:"column:IS_PRIMARY"`
	IndexType string `json:"index_type" gorm:"column:INDEX_TYPE"`
}

// TableDDL holds the full DDL and structured metadata for a table.
type TableDDL struct {
	TableName string       `json:"table_name"`
	Comment   string       `json:"comment,omitempty"`
	RawDDL    string       `json:"ddl"`
	Columns   []ColumnInfo `json:"columns"`
	Indexes   []IndexInfo  `json:"indexes"`
}

// GetTableDDL retrieves DDL, columns, and indexes for a given table.
// Returns (nil, nil) when the table does not exist.
func GetTableDDL(db *gorm.DB, tableName string) (*TableDDL, error) {
	// Check table exists
	var count int64
	err := db.Raw(
		"SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?",
		tableName,
	).Scan(&count).Error
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}

	result := &TableDDL{TableName: tableName}

	// 1. Raw DDL
	var createRow struct {
		Table       string `gorm:"column:Table"`
		CreateTable string `gorm:"column:Create Table"`
	}
	err = db.Raw(fmt.Sprintf("SHOW CREATE TABLE `%s`", tableName)).Scan(&createRow).Error
	if err != nil {
		return nil, fmt.Errorf("SHOW CREATE TABLE 失败: %w", err)
	}
	result.RawDDL = FixUTF8(createRow.CreateTable)

	// 2. Columns
	err = db.Raw(`
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, COLUMN_COMMENT
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`, tableName).Scan(&result.Columns).Error
	if err != nil {
		return nil, fmt.Errorf("查询列信息失败: %w", err)
	}

	// 3. Indexes
	err = db.Raw(`
		SELECT INDEX_NAME,
		       GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) AS COLUMNS,
		       NOT NON_UNIQUE AS IS_UNIQUE,
		       INDEX_NAME = 'PRIMARY' AS IS_PRIMARY,
		       INDEX_TYPE
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		GROUP BY INDEX_NAME, NON_UNIQUE, INDEX_TYPE
	`, tableName).Scan(&result.Indexes).Error
	if err != nil {
		return nil, fmt.Errorf("查询索引信息失败: %w", err)
	}

	// 4. Table comment
	var comment string
	err = db.Raw(`
		SELECT COALESCE(TABLE_COMMENT, '')
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	`, tableName).Scan(&comment).Error
	if err != nil {
		return nil, fmt.Errorf("查询表注释失败: %w", err)
	}
	result.Comment = FixUTF8(comment)

	// Fix double-encoded column comments
	for i := range result.Columns {
		result.Columns[i].Comment = FixUTF8(result.Columns[i].Comment)
	}

	return result, nil
}
