package roundrobin

import (
	"fmt"
	"github.com/apex/log"
	"sync"
	"sync/atomic"
)

type RoundRobiner interface {
	Pick() (*Token, error)
}

func New(tokens []string) RoundRobiner {
	log.Debugf("creating round robin with %d tokens", len(tokens))
	if len(tokens) == 0 {
		return &noTokensRoundRobin{}
	}
	result := make([]*Token, 0, len(tokens))
	for _, token := range tokens {
		result = append(result, NewToken(token))
	}
	return &realRoundRobin{tokens: result}
}

type noTokensRoundRobin struct{}

func (n *noTokensRoundRobin) Pick() (*Token, error) {
	return nil, nil
}

type realRoundRobin struct {
	tokens []*Token
	next   int64
}

func (r *realRoundRobin) Pick() (*Token, error) {
	return r.doPick(0)
}

func (r *realRoundRobin) doPick(try int) (*Token, error) {
	if try > len(r.tokens) {
		return nil, fmt.Errorf("no valid tokens left")
	}
	idx := atomic.LoadInt64(&r.next)
	atomic.StoreInt64(&r.next, (idx+1)%int64(len(r.tokens)))
	if pick := r.tokens[idx]; pick.OK() {
		log.Debugf("picked %s", pick.Key())
		return pick, nil
	}
	return r.doPick(try + 1)
}

func NewToken(token string) *Token {
	return &Token{token: token, valid: true}
}

type Token struct {
	token string
	valid bool
	lock  sync.RWMutex
}

func (t *Token) String() string {
	return t.token[len(t.token)-3:]
}

func (t *Token) Key() string {
	return t.token
}

func (t *Token) OK() bool {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.valid
}

func (t *Token) Invalidate() {
	log.Warnf("invalidate token '...%s.", t)
	t.lock.Lock()
	defer t.lock.Unlock()
	t.valid = false
}
