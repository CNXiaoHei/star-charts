package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/apex/log"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

var (
	errNoMorePages  = errors.New("no more pages to get")
	ErrTooManyStars = errors.New("repo has too many stargazers, github won't allow us to list all stars")
)

type Stargazer struct {
	StarredAt time.Time `json:"starred_at"`
}

func (g *Github) Stargazers(ctx context.Context, repo Repository) (stars []Stargazer, err error) {
	sem := make(chan bool, 4)
	if g.totalPages(repo) > 400 {
		return stars, ErrTooManyStars
	}

	var group errgroup.Group
	var lock sync.Mutex
	for page := 1; page <= g.lastPage(repo); page++ {
		sem <- true
		page := page
		group.Go(func() error {
			defer func() { <-sem }()
			result, err := g.getStargazersPage(ctx, repo, page)
			if errors.Is(err, errNoMorePages) {
				return nil
			}
			if err != nil {
				return err
			}
			lock.Lock()
			defer lock.Unlock()
			stars = append(stars, result...)
			return nil
		})
	}
	err = group.Wait()
	sort.Slice(stars, func(i, j int) bool {
		return stars[i].StarredAt.Before(stars[j].StarredAt)
	})
	return
}

func (g *Github) getStargazersPage(ctx context.Context, repo Repository, page int) ([]Stargazer, error) {
	log := log.WithField("repo", repo.FullName).WithField("page", page)
	defer log.Trace("get page").Stop(nil)

	var stars []Stargazer
	key := fmt.Sprintf("%s_%d", repo.FullName, page)
	etagKey := fmt.Sprintf("%s_%d", repo.FullName, page) + "_etag"
	var etag string
	if err := g.cache.Get(etagKey, &etag); err != nil {
		log.WithError(err).Warnf("failed to get %s from cache", etagKey)
	}
	resp, err := g.makeStarPageRequest(ctx, repo, page, etag)
	if err != nil {
		return stars, err
	}

	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		return stars, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		effectiveEtags.Inc()
		log.Info("not modified")
		err := g.cache.Get(key, &stars)
		if err != nil {
			log.WithError(err).Warnf("failed to get %s from cache", key)
			if err := g.cache.Delete(etagKey); err != nil {
				log.WithError(err).Warnf("failed to delete %s from cache", etagKey)
			}
			return g.getStargazersPage(ctx, repo, page)
		}
		return stars, err
	case http.StatusForbidden:
		rateLimits.Inc()
		log.Warn("rate limit hit")
		return stars, ErrRateLimit
	case http.StatusOK:
		if err := json.Unmarshal(bts, &stars); err != nil {
			return stars, err
		}
		if len(stars) == 0 {
			return stars, errNoMorePages
		}
		if err := g.cache.Put(key, stars); err != nil {
			log.WithError(err).Warnf("failed to cache %s", key)
		}

		etag = resp.Header.Get("etag")
		if etag != "" {
			if err := g.cache.Put(etagKey, etag); err != nil {
				log.WithError(err).Warnf("failed to cache %s", etagKey)
			}
		}
		return stars, nil
	default:
		return stars, fmt.Errorf("%w: %v", ErrGithubAPI, string(bts))
	}
}

func (g *Github) makeStarPageRequest(ctx context.Context, repo Repository, page int, etag string) (*http.Response, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/stargazers?page=%d&per_page=%d", repo.FullName, page, g.pageSize)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/vnd.github.v3.star+json")
	if etag != "" {
		req.Header.Add("If-None-Match", etag)
	}
	return g.authorizedDo(req, 0)
}

func (g *Github) totalPages(repo Repository) int {
	return repo.StargazersCount / g.pageSize
}

func (g *Github) lastPage(repo Repository) int {
	return g.totalPages(repo) + 1
}
