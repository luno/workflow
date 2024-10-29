package frontend

import (
	"embed"
	"html/template"
	"net/http"
)

// Embed the template file
//
//go:embed home.html
var templateFS embed.FS

func HomeHandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := template.ParseFS(templateFS, "home.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = t.Execute(w, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
