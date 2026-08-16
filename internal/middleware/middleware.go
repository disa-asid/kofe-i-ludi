package middleware

import (
	"crypto/subtle"
	"log"
	"net/http"
	"sync"
	"time"
)

// ===== CORS =====

func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ===== Логирование =====

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

// ===== Авторизация админки =====

// AdminAuth проверяет заголовок Authorization: Bearer <token>.
// Сравнение через subtle.ConstantTimeCompare — защита от timing-атак.
func AdminAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				http.Error(w, "admin endpoints disabled: ADMIN_TOKEN not set", http.StatusServiceUnavailable)
				return
			}
			got := r.Header.Get("Authorization")
			want := "Bearer " + token
			if len(got) != len(want) || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ===== Rate limiting =====

// Простой token bucket на IP, без внешних зависимостей.
// Для прод-нагрузки лучше вынести в Redis, но для одной кофейни хватает с головой.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*bucket
	rate     int           // сколько запросов
	per      time.Duration // за какой промежуток
}

type bucket struct {
	tokens   int
	lastSeen time.Time
}

func NewRateLimiter(requests int, per time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*bucket),
		rate:     requests,
		per:      per,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		rl.mu.Lock()
		for ip, b := range rl.visitors {
			if time.Since(b.lastSeen) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.visitors[ip]
	if !ok {
		rl.visitors[ip] = &bucket{tokens: rl.rate - 1, lastSeen: time.Now()}
		return true
	}

	// восполняем токены пропорционально прошедшему времени
	elapsed := time.Since(b.lastSeen)
	refill := int(elapsed / (rl.per / time.Duration(rl.rate)))
	if refill > 0 {
		b.tokens += refill
		if b.tokens > rl.rate {
			b.tokens = rl.rate
		}
		b.lastSeen = time.Now()
	}

	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.allow(ip) {
			http.Error(w, "слишком много запросов, попробуйте позже", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	// За реверс-прокси (nginx/Caddy) стоит читать X-Forwarded-For,
	// но доверять ему можно только если прокси сам его выставляет и
	// не даёт клиенту подделать заголовок напрямую.
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}

// ===== Ограничение размера тела запроса =====

func MaxBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// Chain — удобная склейка middleware в порядке выполнения.
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
