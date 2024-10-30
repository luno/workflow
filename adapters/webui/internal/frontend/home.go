package frontend

import (
	_ "embed"
	"html/template"
	"net/http"
	"strings"
)

// Embed the template file
//
//go:embed home.html
var homeTemplate string

//go:embed main.js
var mainJS string

func HomeHandlerFunc(paths Paths) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := template.New("home.html").Parse(homeTemplate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		jsTemplate, err := template.New("main.js").Parse(mainJS)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Build the JS using the provided paths
		var builder strings.Builder
		err = jsTemplate.Execute(&builder, struct {
			Paths Paths
		}{
			Paths: paths,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = t.Execute(w, struct {
			Paths      Paths
			Javascript template.JS
		}{
			Paths:      paths,
			Javascript: template.JS(builder.String()),
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
