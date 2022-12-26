package github

import (
	"com.github/CNXiaoHei/star-charts/config"
	"com.github/CNXiaoHei/star-charts/internal/cache"
	"com.github/CNXiaoHei/star-charts/internal/roundrobin"
	"github.com/prometheus/client_golang/prometheus"
)

type Github struct {
	tokens          roundrobin.RoundRobiner
	pageSize        int
	cache           *cache.Redis
	maxRateUsagePct int
}

var tokensCount = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "starcharts",
	Subsystem: "github",
	Name:      "available_tokens",
})

func New(config config.Config, cache *cache.Redis) *Github {
	tokensCount.Set(float64(len(config.GithubTokens)))
	return &Github{
		tokens:   roundrobin.New(config.GithubTokens),
		pageSize: config.GithubPageSize,
		cache:    cache,
	}
}
