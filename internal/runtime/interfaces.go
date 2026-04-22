package runtime

import (
	"net/http"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// AppRuntime is the interface implemented by *Server.
// It exists to allow tests and downstream packages to depend on an abstraction
// rather than the concrete Server type.
type AppRuntime interface {
	RenderPage(path string, r *http.Request) (string, error)
	HandleAction(w http.ResponseWriter, r *http.Request, action parser.Page, app *parser.App) error
	HandleAPI(w http.ResponseWriter, r *http.Request, api parser.Page)
}
