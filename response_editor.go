package rewritehtml

import (
	"io"
	"net/http"
	"strings"
	"sync"
)

type ResponseEditor struct {
	http.ResponseWriter
	rewriteFn EditorFunc
	writeOnce sync.Once
	closeOnce sync.Once
	body      io.WriteCloser
}

func (r *ResponseEditor) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

var _ io.WriteCloser = &ResponseEditor{}

// NewResponseEditor will return a ResponseEditor that inspects the http response
// and rewrites the HTML document before passing it to w.
func NewResponseEditor(w http.ResponseWriter, rewriteFn EditorFunc) *ResponseEditor {
	return &ResponseEditor{
		ResponseWriter: w,
		rewriteFn:      rewriteFn,
	}
}

func (r *ResponseEditor) Write(p []byte) (int, error) {
	r.writeOnce.Do(func() {
		header := r.ResponseWriter.Header()

		// TODO: handle content encoding
		if strings.HasPrefix(header.Get("Content-Type"), "text/html") {
			header.Set("Transfer-Encoding", "chunked")
			header.Del("Content-Length")

			r.body = NewTokenEditor(r.ResponseWriter, r.rewriteFn)
		}
	})

	if r.body != nil {
		return r.body.Write(p)
	}

	return r.ResponseWriter.Write(p)
}

func (r *ResponseEditor) Close() (err error) {
	r.closeOnce.Do(func() {
		if r.body != nil {
			err = r.body.Close()
		}
	})

	return
}
