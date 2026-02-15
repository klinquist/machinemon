package server

import (
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     time.Duration
	burst    int
}

type visitor struct {
	tokens   int
	lastSeen time.Time
}

func newRateLimiter(rate time.Duration, burst int) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    burst,
	}
	// Clean up stale entries every minute
	go func() {
		for {
			time.Sleep(time.Minute)
			rl.cleanup()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	if !exists {
		rl.visitors[key] = &visitor{tokens: rl.burst - 1, lastSeen: time.Now()}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := time.Since(v.lastSeen)
	refill := int(elapsed / rl.rate)
	if refill > 0 {
		v.tokens += refill
		if v.tokens > rl.burst {
			v.tokens = rl.burst
		}
		v.lastSeen = time.Now()
	}

	if v.tokens <= 0 {
		return false
	}

	v.tokens--
	return true
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-5 * time.Minute)
	for key, v := range rl.visitors {
		if v.lastSeen.Before(cutoff) {
			delete(rl.visitors, key)
		}
	}
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Real-IP"); fwd != "" {
			ip = fwd
		}
		if !rl.allow(ip) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
