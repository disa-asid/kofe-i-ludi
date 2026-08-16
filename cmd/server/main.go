package main

import (
	"log"
	"net/http"
	"time"

	"cafe-backend/internal/config"
	"cafe-backend/internal/db"
	"cafe-backend/internal/handlers"
	"cafe-backend/internal/middleware"
)

func main() {
	cfg := config.Load()

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer conn.Close()

	reviews := &handlers.ReviewsHandler{DB: conn}
	courses := &handlers.CoursesHandler{DB: conn}
	orders := &handlers.OrdersHandler{DB: conn}

	// Отдельные лимиты: публичная запись — построже (защита от спама и флуда),
	// чтение — посвободнее.
	writeLimiter := middleware.NewRateLimiter(10, time.Minute)
	readLimiter := middleware.NewRateLimiter(120, time.Minute)

	mux := http.NewServeMux()

	// ---- публичные, чтение ----
	mux.Handle("GET /api/reviews", readLimiter.Middleware(http.HandlerFunc(reviews.List)))
	mux.Handle("GET /api/courses", readLimiter.Middleware(http.HandlerFunc(courses.List)))
	mux.Handle("GET /api/orders/{id}", readLimiter.Middleware(http.HandlerFunc(orders.Get)))

	// ---- публичные, запись ----
	mux.Handle("POST /api/reviews", writeLimiter.Middleware(http.HandlerFunc(reviews.Create)))
	mux.Handle("POST /api/courses/{id}/signup", writeLimiter.Middleware(http.HandlerFunc(courses.Signup)))
	mux.Handle("POST /api/orders", writeLimiter.Middleware(http.HandlerFunc(orders.Create)))

	// ---- админка, требует Authorization: Bearer <ADMIN_TOKEN> ----
	adminAuth := middleware.AdminAuth(cfg.AdminToken)
	mux.Handle("GET /api/admin/reviews", adminAuth(http.HandlerFunc(reviews.AdminList)))
	mux.Handle("PATCH /api/admin/reviews/{id}/approve", adminAuth(http.HandlerFunc(reviews.Approve)))
	mux.Handle("DELETE /api/admin/reviews/{id}", adminAuth(http.HandlerFunc(reviews.Delete)))

	mux.Handle("POST /api/admin/courses", adminAuth(http.HandlerFunc(courses.Create)))
	mux.Handle("GET /api/admin/courses/{id}/signups", adminAuth(http.HandlerFunc(courses.AdminSignups)))

	mux.Handle("GET /api/admin/orders", adminAuth(http.HandlerFunc(orders.AdminList)))
	mux.Handle("PATCH /api/admin/orders/{id}/status", adminAuth(http.HandlerFunc(orders.UpdateStatus)))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := middleware.Chain(mux,
		middleware.Logging,
		middleware.CORS(cfg.AllowedOrigin),
		middleware.MaxBody(1<<20), // 1 МБ на запрос — с запасом для любых форм
	)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second, // защита от slowloris
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("сервер запущен на :%s (admin token set: %v)", cfg.Port, cfg.AdminToken != "")
	log.Fatal(srv.ListenAndServe())
}
