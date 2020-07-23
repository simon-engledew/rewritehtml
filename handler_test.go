package rewritehtml

//
import (
	"errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerWithIdentity(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	basicDocument := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/html")

		_, _ = io.WriteString(w, `<html><head><title>Basic Document></head><body>Found</body></html>`)
	})

	handler := Handle(basicDocument, func(r *http.Request) (EditorFunc, error) {
		return func(raw []byte, token *html.Token) (data []byte, done bool) {
			return nil, false
		}, nil
	})

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	require.Equal(t, `<html><head><title>Basic Document></head><body>Found</body></html>`, rr.Body.String())
}

func TestHandlerWithHtml(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	basicDocument := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/html")

		_, _ = io.WriteString(w, `<html><head><title>Basic Document></head><body>Found</body></html>`)
	})

	handler := Handle(basicDocument, func(r *http.Request) (EditorFunc, error) {
		return AfterHead(`<meta name="rewritehtml" content="true" />`), nil
	})

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	require.Equal(t, `<html><head><meta name="rewritehtml" content="true" /><title>Basic Document></head><body>Found</body></html>`, rr.Body.String())
}

func TestHandlerWithError(t *testing.T) {
	errTestFailed := errors.New("test failed")

	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	basicDocument := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/html")

		_, _ = io.WriteString(w, `<html><head><title>Basic Document></head><body>Found</body></html>`)
	})

	handler := Handle(basicDocument, func(r *http.Request) (EditorFunc, error) {
		return AfterHead(`<meta name="rewritehtml" content="true" />`), errTestFailed
	})

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	require.Equal(t, errTestFailed.Error()+"\n", rr.Body.String())
}

func TestHandlerWithJson(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	basicDocument := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content": "moose"}`)
	})

	handler := Handle(basicDocument, func(r *http.Request) (EditorFunc, error) {
		return func(raw []byte, token *html.Token) (data []byte, done bool) {
			panic("function should not have been called for Content-Type application/json")
		}, nil
	})

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	require.Equal(t, `{"content": "moose"}`, rr.Body.String())
}
