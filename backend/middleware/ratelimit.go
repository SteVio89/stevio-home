package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SteVio89/stevio-home/apierr"
)

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// RateLimiter implements per-IP token bucket rate limiting with a background
// cleanup goroutine. Always call Stop() on shutdown to release the goroutine.
//
// State is in-memory and per-process: it does NOT survive a restart (every
// deploy/crash hands each client a fresh budget) and is not shared across
// instances, so limits only hold for a single-process, single-host deployment.
// If you scale horizontally or need brute-force limits to persist across
// restarts, back this with a shared store (Postgres/Redis) instead.
type RateLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*tokenBucket
	rate         float64
	capacity     float64
	trustedProxy bool
	stop         chan struct{}
	stopOnce     sync.Once
}

// NewRateLimiter creates a rate limiter allowing requestsPerMinute per IP.
// It spawns a background goroutine for cleanup; call Stop() on shutdown.
func NewRateLimiter(requestsPerMinute int, trustedProxy bool) *RateLimiter {
	rl := &RateLimiter{
		buckets:      make(map[string]*tokenBucket),
		rate:         float64(requestsPerMinute) / 60.0,
		capacity:     float64(requestsPerMinute),
		trustedProxy: trustedProxy,
		stop:         make(chan struct{}),
	}
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Collect stale IPs under lock, then delete in a second pass
				// to minimize time rl.mu is held (avoids blocking Allow calls).
				var stale []string
				rl.mu.Lock()
				now := time.Now()
				for ip, b := range rl.buckets {
					b.mu.Lock()
					if now.Sub(b.lastRefill) > 30*time.Minute {
						stale = append(stale, ip)
					}
					b.mu.Unlock()
				}
				for _, ip := range stale {
					delete(rl.buckets, ip)
				}
				rl.mu.Unlock()
			case <-rl.stop:
				return
			}
		}
	}()
	return rl
}

// Stop shuts down the background cleanup goroutine. Safe to call multiple times.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stop) })
}

// Allow checks whether a request from the given IP should be allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &tokenBucket{tokens: rl.capacity, lastRefill: time.Now()}
		rl.buckets[ip] = b
	}
	rl.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(rl.capacity, b.tokens+elapsed*rl.rate)
	b.lastRefill = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimit returns a middleware that rate-limits requests using the given RateLimiter.
func RateLimit(rl *RateLimiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := RealIP(r, rl.trustedProxy)
			if !rl.Allow(ip) {
				w.Header().Set("Retry-After", "60")
				apierr.Write(w, apierr.ErrRateLimit())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RealIP extracts the client IP. When trustedProxy is true (behind a
// reverse proxy), X-Real-IP is checked first, then the leftmost entry
// in X-Forwarded-For. Otherwise only RemoteAddr is used since both
// headers can be spoofed by clients hitting the server directly.
func RealIP(r *http.Request, trustedProxy bool) string {
	if trustedProxy {
		if ip := r.Header.Get("X-Real-IP"); ip != "" {
			return stripPort(ip)
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the leftmost (original client) IP.
			if i := strings.IndexByte(xff, ','); i > 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
	}
	return stripPort(r.RemoteAddr)
}

// stripPort removes a port suffix from an address. Handles both
// IPv4 "ip:port" and IPv6 "[ip]:port" formats via net.SplitHostPort.
func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr // bare IP without port
	}
	return host
}
