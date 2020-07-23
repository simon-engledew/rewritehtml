package rewritehtml

import "io"

type TokenEditor struct {
	target    io.Writer
	scanner   *Scanner
	rewriteFn EditorFunc
	done      bool
}

// NewTokenEditor will return a TokenEditor that will inspect each Write call
// and rewrite the HTML document before passing it on to w.
func NewTokenEditor(w io.Writer, rewriteFn EditorFunc) *TokenEditor {
	return &TokenEditor{
		target:    w,
		scanner:   NewScanner(),
		rewriteFn: rewriteFn,
	}
}

func (i *TokenEditor) doWrite(atEOF bool) error {
	for !i.done {
		raw, token, err := i.scanner.Next(atEOF)
		if !atEOF && err == io.ErrNoProgress {
			break
		}
		if err != nil {
			return err
		}

		var data []byte

		data, i.done = i.rewriteFn(raw, token)

		if data == nil {
			data = raw
		}
		_, err = i.target.Write(data)
	}
	if i.done {
		_, _ = io.Copy(i.target, i.scanner.Drain())
	}
	return nil
}

func (i *TokenEditor) Write(p []byte) (int, error) {
	if i.done {
		return i.target.Write(p)
	}
	i.scanner.Concat(p)
	return len(p), i.doWrite(false)
}

func (i *TokenEditor) Close() error {
	return i.doWrite(true)
}
