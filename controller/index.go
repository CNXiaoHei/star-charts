package controller

import (
	"github.com/caarlos0/httperr"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

func Index(fs fs.FS, version string) http.Handler {
	return httperr.NewF(func(w http.ResponseWriter, r *http.Request) error {
		return executeTemplate(fs, w, map[string]string{"Version": version})
	})
}

func executeTemplate(fs fs.FS, w io.Writer, data interface{}) error {
	return template.Must(template.ParseFS(fs, "static/templates/index.html")).Execute(w, data)
}

func HandleForm(fs fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo := strings.TrimPrefix(r.FormValue("repository"), "https://github.com/")
		http.Redirect(w, r, repo, http.StatusSeeOther)
	}
}
