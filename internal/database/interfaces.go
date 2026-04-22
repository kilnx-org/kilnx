package database

// Executor is the interface used by the runtime to execute SQL.
// It allows the runtime to work with a real database or a test fake.
type Executor interface {
	QueryRows(sqlStr string) ([]Row, error)
	QueryRowsWithParams(sqlStr string, params map[string]string) ([]Row, error)
	ExecWithParams(sqlStr string, params map[string]string) error
	BeginTxHandle() (*TxHandle, error)
	Dialect() Dialect
}
