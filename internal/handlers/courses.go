package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"cafe-backend/internal/models"
	"cafe-backend/internal/validate"
)

type CoursesHandler struct {
	DB *sql.DB
}

// GET /api/courses — только курсы в будущем, с расчётом свободных мест.
func (h *CoursesHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, title, description, price, starts_at, seats_total, seats_taken, created_at
		 FROM courses WHERE starts_at >= ? ORDER BY starts_at ASC`,
		time.Now(),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка чтения курсов")
		return
	}
	defer rows.Close()

	list := []models.Course{}
	for rows.Next() {
		var c models.Course
		if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Price, &c.StartsAt, &c.SeatsTotal, &c.SeatsTaken, &c.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "ошибка чтения курсов")
			return
		}
		c.SeatsLeft = c.SeatsTotal - c.SeatsTaken
		list = append(list, c)
	}
	writeJSON(w, http.StatusOK, list)
}

type signupRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// POST /api/courses/{id}/signup
// Места бронируются атомарно одним UPDATE с условием — если два человека
// одновременно жмут "записаться" на последнее место, второй гарантированно
// получит отказ, а не перезапишет счётчик поверх первого (race condition).
func (h *CoursesHandler) Signup(w http.ResponseWriter, r *http.Request) {
	courseID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "некорректный id курса")
		return
	}

	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	req.Name = validate.Trim(req.Name)
	req.Phone = validate.Trim(req.Phone)
	req.Email = validate.Trim(req.Email)

	if err := firstErr(
		validate.NotEmpty("имя", req.Name),
		validate.MaxLen("имя", req.Name, 80),
		validate.Phone(req.Phone),
		validate.Email(req.Email),
	); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}
	defer tx.Rollback() // no-op после успешного Commit

	// Условие seats_taken < seats_total в самом UPDATE — ключевой момент.
	// Если мест нет, RowsAffected будет 0, и мы вернём 409 не вставляя запись.
	res, err := tx.ExecContext(r.Context(),
		`UPDATE courses SET seats_taken = seats_taken + 1
		 WHERE id = ? AND seats_taken < seats_total`,
		courseID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка записи на курс")
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusConflict, "мест не осталось, либо курс не найден")
		return
	}

	insertRes, err := tx.ExecContext(r.Context(),
		`INSERT INTO course_signups (course_id, name, phone, email) VALUES (?, ?, ?, ?)`,
		courseID, req.Name, req.Phone, nullIfEmpty(req.Email),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка записи на курс")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка сервера")
		return
	}

	id, _ := insertRes.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      id,
		"message": "вы записаны на курс",
	})
}

// ==== Админ ====

type createCourseRequest struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       int       `json:"price"`
	StartsAt    time.Time `json:"starts_at"`
	SeatsTotal  int       `json:"seats_total"`
}

// POST /api/admin/courses
func (h *CoursesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	req.Title = validate.Trim(req.Title)
	req.Description = validate.Trim(req.Description)

	if err := firstErr(
		validate.NotEmpty("название", req.Title),
		validate.MaxLen("название", req.Title, 120),
	); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.SeatsTotal <= 0 {
		writeError(w, http.StatusBadRequest, "seats_total должно быть больше 0")
		return
	}

	res, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO courses (title, description, price, starts_at, seats_total) VALUES (?, ?, ?, ?, ?)`,
		req.Title, req.Description, req.Price, req.StartsAt, req.SeatsTotal,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось создать курс")
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// GET /api/admin/courses/{id}/signups — список записавшихся.
func (h *CoursesHandler) AdminSignups(w http.ResponseWriter, r *http.Request) {
	courseID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "некорректный id")
		return
	}

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, course_id, name, phone, COALESCE(email, ''), created_at
		 FROM course_signups WHERE course_id = ? ORDER BY created_at ASC`,
		courseID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка чтения записей")
		return
	}
	defer rows.Close()

	list := []models.CourseSignup{}
	for rows.Next() {
		var s models.CourseSignup
		if err := rows.Scan(&s.ID, &s.CourseID, &s.Name, &s.Phone, &s.Email, &s.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "ошибка чтения записей")
			return
		}
		list = append(list, s)
	}
	writeJSON(w, http.StatusOK, list)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
