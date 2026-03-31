package database

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Row is a single result row as a map of column name to string value
type Row map[string]string

// QueryRows executes a SELECT query and returns all rows as maps
func (db *DB) QueryRows(sql string) ([]Row, error) {
	rows, err := db.conn.Query(sql)
	if err != nil {
		return nil, fmt.Errorf("query error: %w\nSQL: %s", err, sql)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []Row

	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		row := make(Row)
		for i, col := range columns {
			row[col] = fmt.Sprintf("%v", values[i])
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

var paramRe = regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_.]*)`)

// ExecWithParams executes a SQL statement with named parameters from a map.
// Named params like :name, :email are replaced with positional ? params.
func (db *DB) ExecWithParams(sqlStr string, params map[string]string) error {
	query, args := bindParams(sqlStr, params)
	start := time.Now()
	_, err := db.conn.Exec(query, args...)
	if db.OnSlowQuery != nil {
		db.OnSlowQuery(sqlStr, time.Since(start))
	}
	if err != nil {
		return fmt.Errorf("exec error: %w\nSQL: %s\nParams: %v", err, query, args)
	}
	return nil
}

// QueryRowsWithParams executes a SELECT with named params
func (db *DB) QueryRowsWithParams(sqlStr string, params map[string]string) ([]Row, error) {
	query, args := bindParams(sqlStr, params)
	start := time.Now()
	result, err := db.queryRowsInternal(query, args...)
	if db.OnSlowQuery != nil {
		db.OnSlowQuery(sqlStr, time.Since(start))
	}
	return result, err
}

func (db *DB) queryRowsInternal(query string, args ...interface{}) ([]Row, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w\nSQL: %s", err, query)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []Row
	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(Row)
		for i, col := range columns {
			row[col] = fmt.Sprintf("%v", values[i])
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// ExecWithParamsTx executes a mutation within a transaction
func ExecWithParamsTx(tx *sql.Tx, sqlStr string, params map[string]string) error {
	query, args := bindParams(sqlStr, params)
	_, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("exec error: %w\nSQL: %s", err, query)
	}
	return nil
}

// QueryRowsWithParamsTx executes a SELECT within a transaction
func QueryRowsWithParamsTx(tx *sql.Tx, sqlStr string, params map[string]string) ([]Row, error) {
	query, args := bindParams(sqlStr, params)
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w\nSQL: %s", err, query)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []Row
	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(Row)
		for i, col := range columns {
			row[col] = fmt.Sprintf("%v", values[i])
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// bindParams converts :name style params to ? positional params
func bindParams(sqlStr string, params map[string]string) (string, []interface{}) {
	var args []interface{}
	seen := make(map[string]bool)
	_ = seen

	query := paramRe.ReplaceAllStringFunc(sqlStr, func(match string) string {
		name := strings.TrimPrefix(match, ":")
		if val, ok := params[name]; ok {
			args = append(args, val)
			return "?"
		}
		// Leave as-is if param not found (might be a SQL keyword like :memory:)
		return match
	})

	return query, args
}
