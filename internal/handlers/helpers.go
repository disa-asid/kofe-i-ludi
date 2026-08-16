package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// pagination читает limit/offset из query-параметров с безопасными
// границами по умолчанию — чтобы никто не запросил LIMIT 999999999.
func pagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	offset = 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxLimit {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}

// pathID достаёт числовой id из пути через новый роутинг Go 1.22 (r.PathValue).
func pathID(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.PathValue(key), 10, 64)
}
