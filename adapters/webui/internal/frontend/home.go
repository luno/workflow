package frontend

import (
	_ "embed"
	"net/http"
	"text/template"
)

//go:embed home.html
var homeTemplate string

func HomeHandlerFunc(paths Paths) http.HandlerFunc {
	// Use [[ ]] delimiters so JSX {{ }} syntax passes through unmodified.
	t := template.Must(template.New("home.html").Delims("[[", "]]").Parse(homeTemplate))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := t.Execute(w, paths); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

type Paths struct {
	List       string
	Update     string
	ObjectData string
}
