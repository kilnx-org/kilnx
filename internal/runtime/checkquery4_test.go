package runtime

import (
	"fmt"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
)

func TestCheckQuery4(t *testing.T) {
	db, err := database.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	fmt.Printf("db type: %T\n", db.Conn())

	db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)")
	db.Conn().Exec("INSERT INTO users (id, email) VALUES (1, 'queried@example.com')")

	rows, err := db.QueryRowsWithParams("SELECT email FROM users WHERE id = :id", map[string]string{"id": "1"})
	fmt.Printf("err=%v rows=%v\n", err, rows)
}
