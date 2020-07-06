package injecthead // import "github.com/simon-engledew/injecthead"

import (
	"bytes"
	"context"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

var headTag = []byte("head")

// InjectHead inserts data after the first head tag found in r
func InjectHead(w io.Writer, r io.Reader, data string) (n int64, err error) {
	if data == "" {
		return io.Copy(w, r)
	}

	tokenizer := html.NewTokenizer(r)

	var written int

	for {
		tt := tokenizer.Next()

		if tokenizerErr := tokenizer.Err(); tokenizerErr != nil {
			if tokenizerErr != io.EOF {
				err = tokenizerErr
				return
			}
			break
		}

		written, err = w.Write(tokenizer.Raw())

		n += int64(written)

		if err != nil {
			return
		}

		if tt == html.StartTagToken {
			if tag, _ := tokenizer.TagName(); bytes.EqualFold(tag, headTag) {
				written, err = fmt.Fprint(w, data)

				n += int64(written)

				if err != nil {
					return
				}

				written, err = w.Write(tokenizer.Buffered())

				n += int64(written)

				if err != nil {
					return
				}

				var copied int64

				copied, err = io.Copy(w, r)

				n += copied

				return
			}
		}
	}
	return
}

type responsePipe struct {
	Body       chan io.ReadCloser
	StatusCode int
	header     http.Header
	pW         *io.PipeWriter
	pR         *io.PipeReader
}

func NewResponsePipe(header http.Header) *responsePipe {
	pR, pW := io.Pipe()

	return &responsePipe{
		StatusCode: http.StatusOK,
		header:     header,
		Body:       make(chan io.ReadCloser),
		pW:         pW,
		pR:         pR,
	}
}

func (r *responsePipe) Header() http.Header {
	return r.header
}

func (r *responsePipe) Write(p []byte) (int, error) {
	return r.pW.Write(p)
}

func (r *responsePipe) WriteHeader(statusCode int) {
	r.StatusCode = statusCode
	r.Body <- r.pR
}

func (r *responsePipe) Close() (err error) {
	return r.pW.Close()
}

// Handle wraps next in a handler which will insert the result of processRequest at the first
// head tag found in the response body
func Handle(next http.Handler, processRequest func(r *http.Request) (string, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancelFunc := context.WithCancel(r.Context())
		defer cancelFunc()

		r.Header.Set("Accept-Encoding", "identity")

		header := w.Header()

		rp := NewResponsePipe(header)

		done := make(chan struct{})
		go func() {
			defer close(done)

			select {
			case <-ctx.Done():
				// request had no body or was cancelled
				return
			case body := <-rp.Body:
				defer func() {
					if err := body.Close(); err != nil {
						panic(err)
					}
				}()

				// TODO: handle content encoding
				if !strings.HasPrefix(header.Get("Content-Type"), "text/html") {
					w.WriteHeader(rp.StatusCode)
					if _, err := io.Copy(w, body); err != nil {
						panic(err)
					}
					return
				}

				data, err := processRequest(r)
				if err != nil {
					_, _ = io.Copy(ioutil.Discard, body)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				if data == "" {
					w.WriteHeader(rp.StatusCode)
					if _, err := io.Copy(w, body); err != nil {
						panic(err)
					}
					return
				}

				header.Set("Transfer-Encoding", "chunked")
				header.Del("Content-Length")

				w.WriteHeader(rp.StatusCode)

				if _, err := InjectHead(w, body, data); err != nil {
					panic(err)
				}

				return
			}
		}()

		next.ServeHTTP(rp, r)

		_ = rp.Close()

		select {
		case _, _ = <-done:
			break
		}
	})
}
