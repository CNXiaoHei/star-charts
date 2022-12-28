package main

import (
	"com.github/CNXiaoHei/star-charts/config"
	"com.github/CNXiaoHei/star-charts/controller"
	"com.github/CNXiaoHei/star-charts/internal/cache"
	"com.github/CNXiaoHei/star-charts/internal/github"
	"embed"
	"github.com/apex/httplog"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
	"time"
)

//go:embed static/*
var static embed.FS

var version = "devel"

func main() {
	log.SetHandler(text.New(os.Stderr))
	config := config.Get()
	ctx := log.WithField("listen", config.Listen)
	options, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		log.WithError(err).Fatal("invalid redis_url")
	}
	redis := redis.NewClient(options)
	cache := cache.New(redis)
	defer cache.Close()
	github := github.New(config, cache)

	r := mux.NewRouter()
	r.Path("/").Methods(http.MethodGet).Handler(controller.Index(static, version))
	r.Path("/").Methods(http.MethodPost).Handler(controller.HandleForm(static))
	r.PathPrefix("/static/").Methods(http.MethodGet).Handler(http.FileServer(http.FS(static)))
	r.Path("/{owner}/{repo}.svg").Methods(http.MethodGet).Handler(controller.GetRepoChart(github, cache))
	r.Path("/{owner}/{repo}").Methods(http.MethodGet).Handler(controller.GetRepo(static, github, cache, version))

	requestCounter := promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "starcharts",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "total requests",
	}, []string{"code", "method"})
	responseServer := promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "starcharts",
		Subsystem: "http",
		Name:      "responses",
		Help:      "response times and counts",
	}, []string{"code", "method"})

	r.Path("/metrics").Methods(http.MethodGet).Handler(promhttp.Handler())

	srv := &http.Server{
		Handler: httplog.New(
			promhttp.InstrumentHandlerDuration(
				responseServer,
				promhttp.InstrumentHandlerCounter(requestCounter, r),
			),
		),
		Addr:         config.Listen,
		WriteTimeout: 60 * time.Second,
		ReadTimeout:  60 * time.Second,
	}
	ctx.Info("stating up...")
	ctx.WithError(srv.ListenAndServe()).Error("failed to start up server")
}
