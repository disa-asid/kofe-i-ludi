package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"cafe-backend/internal/models"
	"cafe-backend/internal/validate"
)

type ReviewsHandler struct {
	DB *sql.DB
}

type createReviewRequest struct {
	AuthorName string `json:"author_name"`
	Rating     int    `json:"rating"`
	Text       string `json:"text"`
}

// POST /api/reviews — гость оставляет отзыв. Публикуется только после
// одобрения админом (approved=0 по умолчанию).
func (h *ReviewsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	req.AuthorName = validate.Trim(req.AuthorName)
	req.Text = validate.Trim(req.Text)

	if err := firstErr(
		validate.NotEmpty("имя", req.AuthorName),
		validate.MaxLen("имя", req.AuthorName, 80),
		validate.NotEmpty("текст отзыва", req.Text),
		validate.MaxLen("текст отзыва", req.Text, 2000),
		validate.Rating(req.Rating),
	); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Параметризованный запрос — значения передаются отдельно от SQL-текста,
	// драйвер сам их экранирует. Именно это защищает от SQL-инъекций,
	// а не проверки выше (они лишь про качество данных).
	res, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO reviews (author_name, rating, text, approved) VALUES (?, ?, ?, 0)`,
		req.AuthorName, req.Rating, req.Text,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось сохранить отзыв")
		return
	}

	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      id,
		"message": "спасибо! отзыв появится после проверки модератором",
	})
}

// GET /api/reviews?limit=20&offset=0 — только одобренные отзывы.
func (h *ReviewsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r, 20, 100)

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, author_name, rating, text, approved, created_at
		 FROM reviews WHERE approved = 1
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка чтения отзывов")
		return
	}
	defer rows.Close()

	list := []models.Review{}
	for rows.Next() {
		var rv models.Review
		if err := rows.Scan(&rv.ID, &rv.AuthorName, &rv.Rating, &rv.Text, &rv.Approved, &rv.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "ошибка чтения отзывов")
			return
		}
		list = append(list, rv)
	}
	writeJSON(w, http.StatusOK, list)
}

// ==== Админ ====

// GET /api/admin/reviews?status=pending — все отзывы, включая неодобренные.
func (h *ReviewsHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	q := `SELECT id, author_name, rating, text, approved, created_at FROM reviews`
	args := []any{}
	if r.URL.Query().Get("status") == "pending" {
		q += ` WHERE approved = 0`
	}
	q += ` ORDER BY created_at DESC`

	rows, err := h.DB.QueryContext(r.Context(), q, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка чтения отзывов")
		return
	}
	defer rows.Close()

	list := []models.Review{}
	for rows.Next() {
		var rv models.Review
		if err := rows.Scan(&rv.ID, &rv.AuthorName, &rv.Rating, &rv.Text, &rv.Approved, &rv.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "ошибка чтения отзывов")
			return
		}
		list = append(list, rv)
	}
	writeJSON(w, http.StatusOK, list)
}

// PATCH /api/admin/reviews/{id}/approve
func (h *ReviewsHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "некорректный id")
		return
	}
	res, err := h.DB.ExecContext(r.Context(), `UPDATE reviews SET approved = 1 WHERE id = ?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось одобрить отзыв")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "отзыв не найден")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// DELETE /api/admin/reviews/{id}
func (h *ReviewsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "некорректный id")
		return
	}
	res, err := h.DB.ExecContext(r.Context(), `DELETE FROM reviews WHERE id = ?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось удалить отзыв")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "отзыв не найден")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
