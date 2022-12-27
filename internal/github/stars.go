package github

import (
	"context"
	"errors"
	"golang.org/x/sync/errgroup"
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
		g.Go(func() error {
			defer func() {<-sem}()
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
