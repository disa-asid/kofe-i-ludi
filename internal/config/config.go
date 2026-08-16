package config

import "os"

type Config struct {
	Port           string
	DBPath         string
	AdminToken     string // токен для доступа к /api/admin/*
	AllowedOrigin  string // домен фронтенда для CORS, например https://lyudi-i-kofe.ru
}

func Load() Config {
	return Config{
		Port:          getEnv("PORT", "8080"),
		DBPath:        getEnv("DB_PATH", "./cafe.db"),
		AdminToken:    getEnv("ADMIN_TOKEN", ""),
		AllowedOrigin: getEnv("ALLOWED_ORIGIN", "http://localhost:5173"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
