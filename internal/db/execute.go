package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"

	"sql-query/internal/log"
)

// Execute runs a SELECT query inside a read-only transaction with mandatory
// EXPLAIN pre-check. The EXPLAIN blocks DDL (syntax error) and the read-only
// transaction blocks DML (Error 1792).
// Each cell is *string (nil = SQL NULL). timeoutSec <= 0 means no timeout.
// maxRows <= 0 means no limit.
func Execute(gormDB *gorm.DB, sqlContent string, timeoutSec int, maxRows int) ([]string, [][]*string, error) {
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, nil, err
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeoutSec > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	// Start a read-only transaction — MySQL will reject DML (INSERT/UPDATE/DELETE)
	tx, err := sqlDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	// EXPLAIN pre-check (mandatory) — MySQL's EXPLAIN only supports SELECT/DML, not DDL.
	// DDL (DROP/ALTER/CREATE/TRUNCATE) causes a syntax error here,
	// so this blocks DDL at the MySQL parser level, not regex.
	explainRows, err := tx.QueryContext(ctx, "EXPLAIN "+sqlContent)
	if err != nil {
		return nil, nil, fmt.Errorf("SQL 预检失败（可能包含不支持的语句类型）: %w", err)
	}
	explainRows.Close()

	rows, err := tx.QueryContext(ctx, sqlContent)
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
