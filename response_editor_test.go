package rewritehtml

import (
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestWrite(t *testing.T) {
	rr := httptest.NewRecorder()

	editor := NewResponseEditor(rr, AfterHead(`hi`))
	n, err := editor.Write([]byte(`test`))

	require.Equal(t, 4, n)
	require.NoError(t, err)
}
