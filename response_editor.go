package rewritehtml

import (
	"io"
	"net/http"
	"strings"
	"sync"
)

type ResponseEditor struct {
	rewriteFn       EditorFunc
	writeOnce       sync.Once
	writeHeaderOnce sync.Once
	target          http.ResponseWriter
	body            io.WriteCloser
	statusCode      int
}

// NewResponseEditor will return a ResponseEditor that inspects the http response
// and rewrites the HTML document before passing it to w.
func NewResponseEditor(w http.ResponseWriter, rewriteFn EditorFunc) *ResponseEditor {
	return &ResponseEditor{
		target:     w,
		rewriteFn:  rewriteFn,
		statusCode: http.StatusOK,
	}
}

func (r *ResponseEditor) Header() http.Header {
	return r.target.Header()
}

func (r *ResponseEditor) Write(p []byte) (int, error) {
	r.writeOnce.Do(func() {
		header := r.target.Header()

		// TODO: handle content encoding

		if strings.HasPrefix(header.Get("Content-Type"), "text/html") {
			header.Set("Transfer-Encoding", "chunked")
			header.Del("Content-Length")

			r.body = NewTokenEditor(r.target, r.rewriteFn)
		}

		r.writeHeaderOnce.Do(func() {
			r.target.WriteHeader(r.statusCode)
		})
	})

	if r.body != nil {
		return r.body.Write(p)
	}
	return r.target.Write(p)
}

func (r *ResponseEditor) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *ResponseEditor) Close() error {
	if r.body != nil {
		return r.body.Close()
	} else {
		r.writeHeaderOnce.Do(func() {
			r.target.WriteHeader(r.statusCode)
		})
	}
	return nil
}
