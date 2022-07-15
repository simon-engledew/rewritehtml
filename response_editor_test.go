package rewritehtml_test

import (
	"github.com/simon-engledew/rewritehtml"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestWrite(t *testing.T) {
	rr := httptest.NewRecorder()

	editor := rewritehtml.NewResponseEditor(rr, rewritehtml.AfterHead(`hi`))
	n, err := editor.Write([]byte(`test`))

	require.Equal(t, 4, n)
	require.NoError(t, err)
}
