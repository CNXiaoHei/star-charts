package controller

import (
	"com.github/CNXiaoHei/star-charts/internal/cache"
	"com.github/CNXiaoHei/star-charts/internal/github"
	"fmt"
	"github.com/caarlos0/httperr"
	"github.com/gorilla/mux"
	"io/fs"
	"net/http"
)

func GetRepo(fs fs.FS, github *github.Github, cache *cache.Redis, version string) http.Handler {
	return httperr.NewF(func(w http.ResponseWriter, r *http.Request) error {
		name := fmt.Sprintf("%s/%s", mux.Vars(r)["owner"], mux.Vars(r)["repo"])
		details, err = github.RepoDetails(r.Context(), name)
		if err != nil {
			return executeTemplate(fs, w, map[string]error{"Error": err})
		}
		return executeTemplate(fs, w, map[string]interface{}{
			"Version": version,
			"Details": details,
		})
	})
}
