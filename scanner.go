package rewritehtml

import (
	"golang.org/x/net/html"
	"io"
)

// Scanner wraps html.Tokenizer and turns it into a Reader.
type Scanner struct {
	maxBuf      int
	buffer      []byte
	tokenizer   *html.Tokenizer
	previousTag string
}

// NewScanner returns a scanner that is ready to Read an HTML document.
func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) SetMaxBuf(maxBuf int) {
	s.maxBuf = maxBuf
}

// Concat resets the tokenizer and returns any unconsumed data to the buffer.
func (s *Scanner) Concat(p []byte) {
	if s.tokenizer != nil {
		s.buffer = append(s.tokenizer.Buffered(), s.buffer...)
		s.tokenizer = nil
	}
	s.buffer = append(s.buffer, p...)
}

// Buffered returns the remaining available data.
func (s *Scanner) Buffered() int {
	return len(s.buffer)
}

// Read attempts to write as much from the internal buffer to p as possible.
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

// Drain returns a reader that will consume the remaining buffer.
func (s *Scanner) Drain() io.Reader {
	s.Concat([]byte{})
	return NewFragmentReader(s, true)
}

// Next advances the html.Tokenizer and returns the current parse state.
func (s *Scanner) Next(atEOF bool) (raw []byte, token *html.Token, err error) {
	for {
		if s.tokenizer == nil {
			s.tokenizer = html.NewTokenizerFragment(NewFragmentReader(s, atEOF), s.previousTag)
			s.tokenizer.SetMaxBuf(s.maxBuf)
		}

		tt := s.tokenizer.Next()

		if tt == html.ErrorToken {
			nextErr := s.tokenizer.Err()

			if nextErr == io.ErrNoProgress {
				s.Concat([]byte{})
				if atEOF {
					// recreate tokenizer
					continue
				}
			}

			return nil, nil, nextErr
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
