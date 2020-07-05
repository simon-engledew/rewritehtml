package injecthead // import "github.com/simon-engledew/injecthead"

import (
	"bytes"
	"context"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
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

var buffers = sync.Pool{
	New: func() interface{} {
		buffer := new(bytes.Buffer)
		buffer.Grow(32 * 1024)
		return buffer
	},
}

type poolBuffer struct {
	*bytes.Buffer
}

func NewPoolBuffer() *poolBuffer {
	return &poolBuffer{
		getBuffer(),
	}
}

func (buffer *poolBuffer) Close() error {
	buffer.Reset()
	putBuffer(buffer.Buffer)
	return nil
}

func putBuffer(buffer *bytes.Buffer) {
	buffers.Put(buffer)
}

func getBuffer() *bytes.Buffer {
	return buffers.Get().(*bytes.Buffer)
}

type responsePipe struct {
	StatusCode int
	target     http.ResponseWriter
	Body       chan io.ReadCloser
	pW         io.WriteCloser
	pR         io.ReadCloser
}

func NewResponsePipe(target http.ResponseWriter) *responsePipe {
	pR, pW := io.Pipe()

	return &responsePipe{
		StatusCode: http.StatusOK,
		target:     target,
		Body:       make(chan io.ReadCloser),
		pW:         pW,
		pR:         pR,
	}
}

func (r *responsePipe) Header() http.Header {
	return r.target.Header()
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

		rp := NewResponsePipe(w)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()

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

				header := w.Header()

				if !strings.HasPrefix(header.Get("Content-Type"), "text/html") {
					w.WriteHeader(rp.StatusCode)
					_, _ = io.Copy(w, body)
					return
				}

				data, err := processRequest(r)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
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

		if err := rp.Close(); err != nil {
			panic(err)
		}

		wg.Wait()
	})
}

// Wrap a proxy with settings to insert the result of processRequest at the first
// head tag found in each response body
func Wrap(proxy *httputil.ReverseProxy, processRequest func(r *http.Request) (string, error)) *httputil.ReverseProxy {
	proxy2 := new(httputil.ReverseProxy)
	proxy2.Director = func(r *http.Request) {
		r.Header.Set("Accept-Encoding", "identity")
		proxy.Director(r)
	}
	proxy2.ModifyResponse = func(r *http.Response) error {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "text/html") {
			return nil
		}

		data, err := processRequest(r.Request)

		if err != nil {
			return err
		}

		buffer := NewPoolBuffer()

		written, err := InjectHead(buffer, r.Body, data)

		if err != nil {
			// cannot return error, see above
			_ = buffer.Close()
			return err
		}

		if err := r.Body.Close(); err != nil {
			// cannot return error, see above
			_ = buffer.Close()
			return err
		}

		r.ContentLength = written
		r.TransferEncoding = nil

		r.Header.Set("Content-Encoding", "identity")
		r.Header.Set("Content-Length", strconv.FormatInt(written, 10))

		r.Body = buffer

		if proxy.ModifyResponse != nil {
			return proxy.ModifyResponse(r)
		}

		return nil
	}
	return proxy2
}
