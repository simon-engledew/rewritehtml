package injecthead // import "github.com/simon-engledew/injecthead"

import (
	"bytes"
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

type responseBuffer struct {
	header     http.Header
	buffer     *bytes.Buffer
	statusCode int
}

func NewResponseBuffer() *responseBuffer {
	return &responseBuffer{
		header:     make(http.Header),
		buffer:     getBuffer(),
		statusCode: 200,
	}
}

func (r *responseBuffer) Header() http.Header {
	return r.header
}

func (r *responseBuffer) Write(p []byte) (int, error) {
	return r.buffer.Write(p)
}

func (r *responseBuffer) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *responseBuffer) Body() []byte {
	return r.buffer.Bytes()
}

func (r *responseBuffer) Reset() {
	r.buffer.Reset()
	r.header = make(http.Header)
	r.statusCode = 200
}

func (r *responseBuffer) Flush(w http.ResponseWriter) (int64, error) {
	defer putBuffer(r.buffer)
	for k, v := range r.header {
		w.Header()[k] = v
	}
	w.WriteHeader(r.statusCode)
	return io.Copy(w, r.buffer)
}

// Handle wraps next in a handler which will insert the result of processRequest at the first
// head tag found in the response body
func Handle(next http.Handler, processRequest func(r *http.Request) (string, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Accept-Encoding", "identity")

		buffer := NewResponseBuffer()

		next.ServeHTTP(buffer, r)

		if strings.HasPrefix(buffer.Header().Get("Content-Type"), "text/html") {
			data, err := processRequest(r)

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			body := buffer.Body()[:]
			buffer.Reset()
			buffer.buffer.Grow(len(body) + len(data))

			if written, err := InjectHead(buffer, bytes.NewReader(body), data); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else {
				r.Header.Set("Content-Encoding", "identity")
				r.Header.Set("Content-Length", strconv.FormatInt(written, 10))
			}
		}

		if _, err := buffer.Flush(w); err != nil {
			panic(err)
		}
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
