package cache

import (
	rediscache "github.com/go-redis/cache"
	"github.com/go-redis/redis"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

type Redis struct {
	redis *redis.Client
	codec *rediscache.Codec
}

func New(redis *redis.Client) *Redis {
	codec := &rediscache.Codec{
		Redis: redis,
		Marshal: func(v interface{}) ([]byte, error) {
			return msgpack.Marshal(v)
		},
		Unmarshal: func(b []byte, v interface{}) error {
			return msgpack.Unmarshal(b, v)
		},
	}
	return &Redis{
		redis: redis,
		codec: codec,
	}
}

func (r *Redis) Close() error {
	return r.redis.Close()
}
