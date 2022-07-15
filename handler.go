package rewritehtml

import (
	"errors"
	"golang.org/x/net/html"
	"io"
	"net/http"
)

// EditorFunc is called to rewriteFn each token of the document until done.
//
// raw is the current content and token is the parsed representation.
// EditorFunc must not modify or retain raw.
// If data is not nil it will be written to the stream instead of raw.
// When the function returns done the rest of the document will be written
// without being parsed.
type EditorFunc func(raw []byte, token *html.Token) (data []byte, done bool)

var headTag = []byte("head")

// AfterHead returns an EditorFunc that will inject data after the first <head> tag.
func AfterHead(data string) EditorFunc {
	return func(raw []byte, token *html.Token) ([]byte, bool) {
		if token.Type == html.StartTagToken {
			if token.Data == "head" {
				combined := make([]byte, 0, len(raw)+len(data))
				combined = append(combined, raw...)
				combined = append(combined, data...)
				return combined, true
			}
		}
		return raw, false
	}
}

// Handle will rewriteFn any text/html documents that are served by next.
//
// On each request, Handle will call processRequest to provide an EditorFunc.
// The response will be intercepted and EditorFunc will be applied if the Content-Type is text/html.
// If the Content-Type is not text/html EditorFunc will not be called.
// Handle does not spawn any additional goroutines and will attempt to use the smallest buffer possible
// to read and edit the document.
func Handle(next http.Handler, processRequest func(r *http.Request) (EditorFunc, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Accept-Encoding", "identity")

		fn, err := processRequest(r)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		editor := NewResponseEditor(w, fn)

		next.ServeHTTP(editor, r)

		// the response has been partially sent and cannot be recovered
		// the only thing left to do is to panic
		if err := editor.Close(); err != nil && !errors.Is(err, io.EOF) {
			panic(err)
		}
	})
}
