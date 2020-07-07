package injecthead // import "github.com/simon-engledew/injecthead"

import (
	"golang.org/x/net/html"
	"io"
	"net/http"
	"strings"
	"sync"
)

var headTag = []byte("head")

type Scanner struct {
	buffer      []byte
	tokenizer   *html.Tokenizer
	previousTag string
}

func (s *Scanner) Concat(p []byte) {
	if s.tokenizer != nil {
		s.buffer = append(s.buffer, s.tokenizer.Buffered()...)
	}
	s.buffer = append(s.buffer, p...)
	s.tokenizer = nil
}

type ScannerState interface {
	Raw() []byte
	Err() error
	TagName() (name []byte, hasAttr bool)
	Token() html.Token
}

type BufferReader interface {
	io.Reader
	Buffered() int
}

type FragmentReader struct {
	r   BufferReader
	eof bool
}

func NewFragmentReader(r BufferReader, eof bool) *FragmentReader {
	return &FragmentReader{
		r,
		eof,
	}
}

func (s *FragmentReader) Read(p []byte) (int, error) {
	size := len(p)
	have := s.r.Buffered()
	read := have

	if !s.eof && size >= have {
		return 0, io.ErrNoProgress
	}

	if have == 0 {
		if s.eof {
			return 0, io.EOF
		}
		return 0, io.ErrNoProgress
	}

	if size < have {
		read = size
	}

	var err error
	if s.eof && read == have {
		err = io.EOF
	}

	_, _ = s.r.Read(p[:read])

	return read, err
}

func (s *Scanner) Buffered() int {
	return len(s.buffer)
}

func (s *Scanner) Drain() io.Reader {
	s.Concat([]byte{})
	return NewFragmentReader(s, true)
}

func (s *Scanner) Read(p []byte) (int, error) {
	size := len(p)
	have := len(s.buffer)
	read := have

	if size < have {
		read = size
	}

	if have == 0 {
		return 0, io.ErrNoProgress
	}

	copy(p, s.buffer[:read])
	s.buffer = s.buffer[read:]
	return read, nil
}

func (s *Scanner) Next(atEOF bool) ([]byte, *html.Token, error) {
	for {
		if s.tokenizer == nil {
			s.tokenizer = html.NewTokenizerFragment(NewFragmentReader(s, atEOF), s.previousTag)
		}

		tt := s.tokenizer.Next()

		if tt == html.ErrorToken {
			err := s.tokenizer.Err()
			if err == io.ErrNoProgress {
				s.Concat([]byte{})

				if atEOF {
					// recreate tokenizer
					continue
				}
			}
			return nil, nil, err
		}

		raw := s.tokenizer.Raw()
		token := s.tokenizer.Token()
		if tt == html.StartTagToken {
			s.previousTag = token.Data
		}
		if tt == html.EndTagToken {
			s.previousTag = ""
		}

		return raw, &token, nil
	}
}

func NewScanner() *Scanner {
	return &Scanner{}
}

type EditorFunc func(raw string, token *html.Token) (data string, done bool)

type tokenEditor struct {
	target  io.Writer
	scanner *Scanner
	rewrite EditorFunc
	done    bool
}

func NewTokenEditor(w io.Writer, rewrite EditorFunc) *tokenEditor {
	return &tokenEditor{
		target:  w,
		scanner: NewScanner(),
		rewrite: rewrite,
	}
}

func (i *tokenEditor) doWrite(atEOF bool) error {
	for {
		raw, token, err := i.scanner.Next(atEOF)
		if !atEOF && err == io.ErrNoProgress {
			break
		}
		if err != nil {
			return err
		}

		var data string

		data, i.done = i.rewrite(string(raw), token)

		_, err = i.target.Write([]byte(data))

		if i.done {
			_, _ = io.Copy(i.target, i.scanner.Drain())
		}
	}
	return nil
}

func (i *tokenEditor) Write(p []byte) (int, error) {
	if i.done {
		return i.target.Write(p)
	}
	i.scanner.Concat(p)
	return len(p), i.doWrite(false)
}

func (i *tokenEditor) Close() error {
	return i.doWrite(true)
}

func AfterHead(data string) EditorFunc {
	return func(raw string, token *html.Token) (string, bool) {
		if token.Type == html.StartTagToken {
			if token.Data == "head" {
				return raw + data, true
			}
		}
		return raw, false
	}
}

type ResponseEditor struct {
	rewrite    EditorFunc
	once       sync.Once
	target     http.ResponseWriter
	body       io.WriteCloser
	statusCode int
}

func NewResponseEditor(w http.ResponseWriter, rewrite EditorFunc) *ResponseEditor {
	return &ResponseEditor{
		target:     w,
		rewrite:    rewrite,
		statusCode: http.StatusOK,
	}
}

func (r *ResponseEditor) Header() http.Header {
	return r.target.Header()
}

func (r *ResponseEditor) Write(p []byte) (int, error) {
	r.once.Do(func() {
		header := r.target.Header()

		// TODO: handle content encoding

		if strings.HasPrefix(header.Get("Content-Type"), "text/html") {
			header.Set("Transfer-Encoding", "chunked")
			header.Del("Content-Length")

			r.body = NewTokenEditor(r.target, r.rewrite)
		}

		r.target.WriteHeader(r.statusCode)
	})
	if r.body != nil {
		return r.body.Write(p)
	}
	if r.target != nil {
		return r.target.Write(p)
	}
	return 0, io.ErrClosedPipe
}

func (r *ResponseEditor) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *ResponseEditor) Close() error {
	if r.body != nil {
		return r.body.Close()
	}
	return nil
}

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

		editor.Close()
	})
}
