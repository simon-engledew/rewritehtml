package rewritehtml

import (
	"io"
)

type BufferedReader interface {
	io.Reader
	Buffered() int
}

type FragmentReader struct {
	r   BufferedReader
	eof bool
}

// NewFragmentReader returns a FragmentReader that will read from r but
// only return an io.EOF error if eof is true.
// If eof is false the reader will return io.ErrNoProgress to indicate
// that there could be more data on the stream but it cannot be read at the moment.
func NewFragmentReader(r BufferedReader, eof bool) *FragmentReader {
	return &FragmentReader{
		r,
		eof,
	}
}

func (fr *FragmentReader) Read(p []byte) (int, error) {
	size := len(p)
	have := fr.r.Buffered()
	read := have

	if !fr.eof && size > have {
		return 0, io.ErrNoProgress
	}

	if have == 0 {
		if fr.eof {
			return 0, io.EOF
		}
		return 0, io.ErrNoProgress
	}

	if size < have {
		read = size
	}

	var err error

	n, err := fr.r.Read(p[:read])

	if err == nil && fr.eof && read == have {
		err = io.EOF
	}

	return n, err
}
