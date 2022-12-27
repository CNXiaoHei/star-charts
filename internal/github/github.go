package github

import (
	"com.github/CNXiaoHei/star-charts/config"
	"com.github/CNXiaoHei/star-charts/internal/cache"
	"com.github/CNXiaoHei/star-charts/internal/roundrobin"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/apex/log"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"net/http"
)

var ErrRateLimit = errors.New("rate limited, please try again later")

var ErrGithubAPI = errors.New("failed to talk with github api")

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

var rateLimits = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "starcharts",
	Subsystem: "github",
	Name:      "rate_limit_hits_total",
})

var effectiveEtags = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "starcharts",
	Subsystem: "github",
	Name:      "effective_etag_uses_total",
})

var invalidatedTokens = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "starcharts",
	Subsystem: "github",
	Name:      "invalidated_tokens_total",
})

var rateLimiters = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "starcharts",
	Subsystem: "github",
	Name:      "rate_limit_remaining",
}, []string{"token"})

func init() {
	prometheus.MustRegister(rateLimits, effectiveEtags, invalidatedTokens, tokensCount, rateLimiters)
}

func New(config config.Config, cache *cache.Redis) *Github {
	tokensCount.Set(float64(len(config.GithubTokens)))
	return &Github{
		tokens:   roundrobin.New(config.GithubTokens),
		pageSize: config.GithubPageSize,
		cache:    cache,
	}
}

const maxTries = 3

func (g *Github) authorizedDo(req *http.Request, try int) (*http.Response, error) {
	if try > maxTries {
		return nil, fmt.Errorf("couldn't find a valid token")
	}
	token, err := g.tokens.Pick()
	if err != nil || token == nil {
		log.WithError(err).Warnf("couldn't find a valid token")
		return http.DefaultClient.Do(req)
	}

	if err := g.checkToken(token); err != nil {
		log.WithError(err).Warnf("couldn't check rate limit, trying again")
		return g.authorizedDo(req, try+1)
	}

	req.Header.Add("Authorization", fmt.Sprintf("token %s", token.Key()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, err
	}
	return resp, err
}

func (g *Github) checkToken(token *roundrobin.Token) error {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/rate_limit", nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("token %s", token.Key()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		token.Invalidate()
		invalidatedTokens.Inc()
		return fmt.Errorf("token is invalid")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var limit rateLimit
	if err := json.Unmarshal(bts, &limit); err != nil {
		return err
	}
	rate := limit.Rate
	log.Debugf("%s rate %d/%d", token, rate.Remaining, rate.Limit)
	rateLimiters.WithLabelValues(token.String()).Set(float64(rate.Remaining))
	if isAboveTargetUsage(rate, g.maxRateUsagePct) {
		return fmt.Errorf("token usage is to high: %d/%d", rate.Remaining, rate.Limit)
	}
	return nil
}

func isAboveTargetUsage(rate rate, target int) bool {
	return rate.Remaining*100/rate.Limit < target
}

type rateLimit struct {
	Rate rate `json:"rate"`
}

type rate struct {
	Remaining int `json:"remaining"`
	Limit     int `json:"limit"`
}
