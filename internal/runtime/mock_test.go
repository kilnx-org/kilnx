package runtime

import (
	"fmt"
	"strings"

	"github.com/kilnx-org/kilnx/internal/database"
)

var _ database.Executor = (*mockExecutor)(nil)

// mockExecutor is a test fake for database.Executor
type mockExecutor struct {
	queryRowsResults           map[string][]database.Row
	queryRowsWithParamsResults map[string][]database.Row
	queryRowsWithParamsErr     map[string]error
	queryRowsErr               map[string]error
	execErr                    map[string]error
	execCalled                 []execCall
	queryRowsCalls             []string // SQL strings executed via QueryRows
	queryRowsWithParamsCalls   []string // SQL strings executed via QueryRowsWithParams
}

type execCall struct {
	SQL    string
	Params map[string]string
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		queryRowsResults:           make(map[string][]database.Row),
		queryRowsWithParamsResults: make(map[string][]database.Row),
		queryRowsWithParamsErr:     make(map[string]error),
		queryRowsErr:               make(map[string]error),
		execErr:                    make(map[string]error),
		execCalled:                 nil,
	}
}

func (m *mockExecutor) QueryRows(sqlStr string) ([]database.Row, error) {
	m.queryRowsCalls = append(m.queryRowsCalls, sqlStr)
	if err, ok := m.queryRowsErr[sqlStr]; ok {
		return nil, err
	}
	if rows, ok := m.queryRowsResults[sqlStr]; ok {
		return rows, nil
	}
	return nil, nil
}

func (m *mockExecutor) QueryRowsWithParams(sqlStr string, params map[string]string) ([]database.Row, error) {
	m.queryRowsWithParamsCalls = append(m.queryRowsWithParamsCalls, sqlStr)
	key := sqlStr + "|" + paramsKey(params)
	if err, ok := m.queryRowsWithParamsErr[key]; ok {
		return nil, err
	}
	if err, ok := m.queryRowsWithParamsErr[sqlStr]; ok {
		return nil, err
	}
	if rows, ok := m.queryRowsWithParamsResults[key]; ok {
		return rows, nil
	}
	// Fallback: match by SQL only
	if rows, ok := m.queryRowsWithParamsResults[sqlStr]; ok {
		return rows, nil
	}
	return nil, nil
}

func (m *mockExecutor) ExecWithParams(sqlStr string, params map[string]string) error {
	m.execCalled = append(m.execCalled, execCall{SQL: sqlStr, Params: params})
	key := sqlStr + "|" + paramsKey(params)
	if err, ok := m.execErr[key]; ok {
		return err
	}
	if err, ok := m.execErr[sqlStr]; ok {
		return err
	}
	return nil
}

func (m *mockExecutor) BeginTxHandle() (*database.TxHandle, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockExecutor) Dialect() database.Dialect {
	return &database.SqliteDialect{}
}

func paramsKey(params map[string]string) string {
	var parts []string
	for k, v := range params {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}
