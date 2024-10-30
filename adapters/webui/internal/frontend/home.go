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

func HomeHandlerFunc(paths Paths) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := template.ParseFS(templateFS, "home.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = t.Execute(w, struct {
			Paths Paths
		}{
			Paths: paths,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

type Paths struct {
	List       string
	Update     string
	ObjectData string
}
