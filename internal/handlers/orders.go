package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"cafe-backend/internal/models"
	"cafe-backend/internal/validate"
)

type OrdersHandler struct {
	DB *sql.DB
}

type createOrderRequest struct {
	CustomerName string             `json:"customer_name"`
	Phone        string             `json:"phone"`
	PickupTime   time.Time          `json:"pickup_time"`
	Items        []models.OrderItem `json:"items"`
}

const (
	maxOrderItems = 30
	maxItemQty    = 20
)

// POST /api/orders — предзаказ на самовывоз.
func (h *OrdersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	req.CustomerName = validate.Trim(req.CustomerName)
	req.Phone = validate.Trim(req.Phone)

	if err := firstErr(
		validate.NotEmpty("имя", req.CustomerName),
		validate.MaxLen("имя", req.CustomerName, 80),
		validate.Phone(req.Phone),
	); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "заказ пуст")
		return
	}
	if len(req.Items) > maxOrderItems {
		writeError(w, http.StatusBadRequest, "слишком много позиций в заказе")
		return
	}

	total := 0
	for _, it := range req.Items {
		if validate.Trim(it.Name) == "" {
			writeError(w, http.StatusBadRequest, "у позиции в заказе пустое название")
			return
		}
		if it.Qty <= 0 || it.Qty > maxItemQty {
			writeError(w, http.StatusBadRequest, "некорректное количество для позиции "+it.Name)
			return
		}
		if it.Price < 0 {
			writeError(w, http.StatusBadRequest, "некорректная цена для позиции "+it.Name)
			return
		}
		total += it.Price * it.Qty
	}

	if req.PickupTime.Before(time.Now()) {
		writeError(w, http.StatusBadRequest, "время самовывоза не может быть в прошлом")
		return
	}

	itemsJSON, err := json.Marshal(req.Items)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка обработки заказа")
		return
	}

	res, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO orders (customer_name, phone, pickup_time, items_json, total_price, status)
		 VALUES (?, ?, ?, ?, ?, 'new')`,
		req.CustomerName, req.Phone, req.PickupTime, string(itemsJSON), total,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось создать заказ")
		return
	}

	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          id,
		"total_price": total,
		"status":      "new",
	})
}

// GET /api/orders/{id}?phone=... — проверка статуса своего заказа.
// Требуем совпадения телефона, чтобы нельзя было листать чужие заказы по id.
func (h *OrdersHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "некорректный id")
		return
	}
	phone := validate.Trim(r.URL.Query().Get("phone"))
	if phone == "" {
		writeError(w, http.StatusBadRequest, "укажите телефон, указанный при заказе")
		return
	}

	var o models.Order
	var itemsJSON string
	err = h.DB.QueryRowContext(r.Context(),
		`SELECT id, customer_name, phone, pickup_time, items_json, total_price, status, created_at
		 FROM orders WHERE id = ? AND phone = ?`,
		id, phone,
	).Scan(&o.ID, &o.CustomerName, &o.Phone, &o.PickupTime, &itemsJSON, &o.TotalPrice, &o.Status, &o.CreatedAt)

	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "заказ не найден")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка чтения заказа")
		return
	}
	json.Unmarshal([]byte(itemsJSON), &o.Items)

	writeJSON(w, http.StatusOK, o)
}

// ==== Админ ====

// GET /api/admin/orders?status=new
func (h *OrdersHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	q := `SELECT id, customer_name, phone, pickup_time, items_json, total_price, status, created_at FROM orders`
	args := []any{}
	if status := r.URL.Query().Get("status"); status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC`

	rows, err := h.DB.QueryContext(r.Context(), q, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка чтения заказов")
		return
	}
	defer rows.Close()

	list := []models.Order{}
	for rows.Next() {
		var o models.Order
		var itemsJSON string
		if err := rows.Scan(&o.ID, &o.CustomerName, &o.Phone, &o.PickupTime, &itemsJSON, &o.TotalPrice, &o.Status, &o.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "ошибка чтения заказов")
			return
		}
		json.Unmarshal([]byte(itemsJSON), &o.Items)
		list = append(list, o)
	}
	writeJSON(w, http.StatusOK, list)
}

var validStatuses = map[string]bool{
	"new": true, "confirmed": true, "ready": true, "completed": true, "cancelled": true,
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

// PATCH /api/admin/orders/{id}/status
func (h *OrdersHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "некорректный id")
		return
	}
	var req updateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if !validStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "недопустимый статус")
		return
	}

	res, err := h.DB.ExecContext(r.Context(), `UPDATE orders SET status = ? WHERE id = ?`, req.Status, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось обновить статус")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "заказ не найден")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": req.Status})
}
