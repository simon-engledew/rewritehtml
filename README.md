![Go](https://github.com/simon-engledew/rewritehtml/workflows/Go/badge.svg)

### rewritehtml

Alter the HTML response body of a http.Handler.

#### Inject CSRF tokens into static single page webapps.

Useful if you have server side rendered a React application and want to access a CSRF token without making an additional remote request.

Can be run as a `httputil.SingleHostReverseProxy` in front of a webserver like Nginx, or as a `http.Handler` wrapping another `http.Handler`, e.g: `http.FileServer`.

```golang
package main

import (
	"errors"
	"fmt"
	"github.com/gorilla/csrf"
	"github.com/simon-engledew/rewritehtml"
	"net/http"
	"net/http/httputil"
	"net/url"
	"text/template"
)

func csrfMeta(r *http.Request) (rewritehtml.EditorFunc, error) {
	token := csrf.Token(r)

	if token == "" {
		return nil, errors.New("no csrf middleware installed")
	}

	return rewritehtml.AfterHead(fmt.Sprintf(`<meta name="csrf" content="%s" />`, template.HTMLEscapeString(token))), nil
}

func main() {
	proxy := rewritehtml.Handle(httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:4000",
	}), csrfMeta)

	protect := csrf.Protect(
		[]byte(`32-byte-long-auth-key`),
		csrf.Secure(true),
		csrf.FieldName("csrf_token"),
		csrf.RequestHeader("X-CSRF-TOKEN"),
		csrf.Path("/"),
		csrf.CookieName("csrf-token"),
		csrf.MaxAge(0),
	)

	http.ListenAndServe("127.0.0.1:3000", protect(proxy))
}
```

The JavaScript application can then access the CSRF token with something like:

```javascript
function csrf() {
  const element = document.querySelector('meta[name="csrf"]');
  return element && element.getAttribute('content');
}
```
