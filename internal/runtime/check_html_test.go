package runtime

import (
	"github.com/kilnx-org/kilnx/internal/parser"
	"net/http/httptest"
	"testing"
)

func TestCheckHTML(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeHTML, HTMLContent: `<div>hello</div>`},
	}, map[string]string{}, s.getApp())
	t.Logf("code=%d body=%q", rec.Code, rec.Body.String())
}
