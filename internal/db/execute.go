package db

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"

	"sql-query/internal/log"
)

// Execute runs a SQL query and returns column names and rows.
// Each cell is *string (nil = SQL NULL). timeoutSec <= 0 means no timeout.
// maxRows <= 0 means no limit.
func Execute(db *gorm.DB, sqlContent string, timeoutSec int, maxRows int) ([]string, [][]*string, error) {
	var rows *sql.Rows
	var err error

	if timeoutSec > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		defer cancel()
		rows, err = db.WithContext(ctx).Raw(sqlContent).Rows()
	} else {
		rows, err = db.Raw(sqlContent).Rows()
	}
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var data [][]*string
	rowCount := 0
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		scanArgs := make([]interface{}, len(columns))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, nil, err
		}

		row := make([]*string, len(columns))
		for i, v := range values {
			if v.Valid {
				s := v.String
				row[i] = &s
			}
		}
		data = append(data, row)

		rowCount++
		if rowCount%1000 == 0 {
			log.Debug("已读取 %d 行...", rowCount)
		}

		if maxRows > 0 && rowCount >= maxRows {
			log.Warn("已达到最大行数限制 (%d 行)，结果被截断", maxRows)
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return columns, data, nil
}
