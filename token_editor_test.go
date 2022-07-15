package rewritehtml_test

import (
	"bytes"
	"github.com/simon-engledew/rewritehtml"
	"github.com/stretchr/testify/require"
	"io"
	"strings"
	"testing"
)

func testWrites(w io.WriteCloser, values ...string) error {
	for _, value := range values {
		_, err := io.WriteString(w, value)
		if err != nil {
			return err
		}
	}
	return w.Close()
}

func TestMiss(t *testing.T) {
	output := new(bytes.Buffer)

	editor := rewritehtml.NewTokenEditor(output, rewritehtml.AfterHead(`<meta name="rewritehtml" content="true" />`))

	require.ErrorIs(t, testWrites(editor, `<html><p></p><pre>`, `moose</pre></html>`), io.EOF)

	require.Equal(t, `<html><p></p><pre>moose</pre></html>`, output.String())
}

func TestHit(t *testing.T) {
	output := new(bytes.Buffer)

	editor := rewritehtml.NewTokenEditor(output, rewritehtml.AfterHead(`<meta name="rewritehtml" content="true" />`))

	require.NoError(t, testWrites(editor, `<html><head></head><pre>`, `moose</pre></html>`))

	require.Equal(t, `<html><head><meta name="rewritehtml" content="true" /></head><pre>moose</pre></html>`, output.String())
}

func TestShortCircuitWrite(t *testing.T) {
	output := new(bytes.Buffer)

	zeros := strings.Repeat(`0`, 1024)
	script := strings.Repeat(`var moose; `, 512)

	editor := rewritehtml.NewTokenEditor(output, rewritehtml.AfterHead(`<meta name="rewritehtml" content="true" />`))

	require.NoError(t, testWrites(editor, `<!DOCTYPE html><html><head>`, `<link rel="icon" type="image/png" href="data:image/png;base64,`+zeros+`</link></head><script>`+script+`</script>`, `<script>`+script+`</script>`))

	require.Equal(t, `<!DOCTYPE html><html><head><meta name="rewritehtml" content="true" /><link rel="icon" type="image/png" href="data:image/png;base64,`+zeros+`</link></head><script>`+script+`</script><script>`+script+`</script>`, output.String())
}

func TestShortWrite(t *testing.T) {
	output := new(bytes.Buffer)

	editor := rewritehtml.NewTokenEditor(output, rewritehtml.AfterHead(`<meta name="rewritehtml" content="true" />`))

	require.NoError(t, testWrites(editor, `<he`, `ad></head><pre>`, `moose</pre>`))

	require.Equal(t, `<head><meta name="rewritehtml" content="true" /></head><pre>moose</pre>`, output.String())
}

func TestConcat(t *testing.T) {
	output := new(bytes.Buffer)

	zeros := strings.Repeat(`0`, 1024)
	script := strings.Repeat(`var moose; `, 512)

	editor := rewritehtml.NewTokenEditor(output, rewritehtml.AfterHead(`<meta name="rewritehtml" content="true" />`))

	require.NoError(t, testWrites(editor, `<!DOCTYPE html><html><head><link rel="icon" type="image/png" href="data:image/png;base64,`+zeros, `</link></head><script>`+script+`</script>`))

	require.Equal(t, `<!DOCTYPE html><html><head><meta name="rewritehtml" content="true" /><link rel="icon" type="image/png" href="data:image/png;base64,`+zeros+`</link></head><script>`+script+`</script>`, output.String())
}

func TestCDataWrite(t *testing.T) {
	output := new(bytes.Buffer)

	editor := rewritehtml.NewTokenEditor(output, rewritehtml.AfterHead(`<meta name="rewritehtml" content="true" />`))

	require.NoError(t, testWrites(editor, `<script>`, `javascript {} <head></head>`, `moose</script><head></head>`))

	require.Equal(t, `<script>javascript {} <head></head>moose</script><head><meta name="rewritehtml" content="true" /></head>`, output.String())
}
