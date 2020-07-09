package injecthead

//
import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMiss(t *testing.T) {
	output := new(bytes.Buffer)

	editor := NewTokenEditor(output, AfterHead(`<meta name="injecthead" content="true" />`))
	editor.Write([]byte(`<html><p></p><pre>`))
	editor.Write([]byte(`moose</pre></html>`))
	editor.Close()

	assert.Equal(t, `<html><p></p><pre>moose</pre></html>`, output.String())
}

func TestHit(t *testing.T) {
	output := new(bytes.Buffer)

	editor := NewTokenEditor(output, AfterHead(`<meta name="injecthead" content="true" />`))
	editor.Write([]byte(`<html><head></head><pre>`))
	editor.Write([]byte(`moose</pre></html>`))
	editor.Close()

	assert.Equal(t, `<html><head><meta name="injecthead" content="true" /></head><pre>moose</pre></html>`, output.String())
}

func TestShortWrite(t *testing.T) {
	output := new(bytes.Buffer)

	editor := NewTokenEditor(output, AfterHead(`<meta name="injecthead" content="true" />`))
	editor.Write([]byte(`<he`))
	editor.Write([]byte(`ad></head><pre>`))
	editor.Write([]byte(`moose</pre>`))
	editor.Close()

	assert.Equal(t, `<head><meta name="injecthead" content="true" /></head><pre>moose</pre>`, output.String())
}

func TestCDataWrite(t *testing.T) {
	output := new(bytes.Buffer)

	editor := NewTokenEditor(output, AfterHead(`<meta name="injecthead" content="true" />`))
	editor.Write([]byte(`<script>`))
	editor.Write([]byte(`javascript {} <head></head>`))
	editor.Write([]byte(`moose</script><head></head>`))
	editor.Close()

	assert.Equal(t, `<script>javascript {} <head></head>moose</script><head><meta name="injecthead" content="true" /></head>`, output.String())
}

//func meta(r *http.Request) (EditorFunc, error) {
//	return AfterHead(fmt.Sprintf(`<meta name="injecthead" content="%s" />`, template.HTMLEscapeString("injected"))), nil
//}
//
//func TestWrap(t *testing.T) {
//	fs := Handle(http.FileServer(http.Dir(".")), meta)
//
//	mux := http.NewServeMux()
//	mux.Handle("/", fs)
//	http.ListenAndServe("127.0.0.1:3000", mux)
//}
