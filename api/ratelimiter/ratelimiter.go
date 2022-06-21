package ratelimiter

import (
	"sync"
	"time"

	"github.com/nknorg/nkn/v2/common"
	"golang.org/x/time/rate"
)

var rateLimiters = common.NewGoCache(10*time.Minute, 5*time.Minute)
var lock sync.Mutex

func GetLimiter(key string, limit float64, burst int) *rate.Limiter {
	lock.Lock()
	defer lock.Unlock()

	if limiter, ok := rateLimiters.Get([]byte(key)); ok {
		return limiter.(*rate.Limiter)
	}

	limiter := rate.NewLimiter(rate.Limit(limit), burst)
	rateLimiters.Set([]byte(key), limiter)

	return limiter
}
