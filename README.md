### injecthead

Inject CSRF tokens into static single page webapps.

```golang
func csrfMeta(r *http.Request) (string, error) {
    token := csrf.Token(r.Request)
    
    if token == "" {
        return "", errors.New("no csrf middleware installed")
    }
    
    return fmt.Sprintf(`<meta name="csrf" content="%s" />`, template.EscapeHTMLString(token)), nil
}

fs := Handle(http.FileServer(http.Dir(".")), meta)

proxy := httputil.NewSingleHostReverseProxy(&url.URL{
    Scheme: "http",
    Host:   "127.0.0.1:4000",
})

mux := http.NewServeMux()
mux.Handle("/static", http.StripPrefix("/static", fs))
mux.Handle("/hmr", http.StripPrefix("/hmr", proxy))

http.ListenAndServe("127.0.0.1:3000", mux)
``` 