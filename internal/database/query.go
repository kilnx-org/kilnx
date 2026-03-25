package database

import (
	"fmt"
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
