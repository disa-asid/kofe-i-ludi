package models

import "time"

type Review struct {
	ID         int64     `json:"id"`
	AuthorName string    `json:"author_name"`
	Rating     int       `json:"rating"`
	Text       string    `json:"text"`
	Approved   bool      `json:"approved"`
	CreatedAt  time.Time `json:"created_at"`
}

type Course struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       int       `json:"price"`
	StartsAt    time.Time `json:"starts_at"`
	SeatsTotal  int       `json:"seats_total"`
	SeatsTaken  int       `json:"seats_taken"`
	SeatsLeft   int       `json:"seats_left"`
	CreatedAt   time.Time `json:"created_at"`
}

type CourseSignup struct {
	ID        int64     `json:"id"`
	CourseID  int64     `json:"course_id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type OrderItem struct {
	Name  string `json:"name"`
	Qty   int    `json:"qty"`
	Price int    `json:"price"`
}

type Order struct {
	ID           int64       `json:"id"`
	CustomerName string      `json:"customer_name"`
	Phone        string      `json:"phone"`
	PickupTime   time.Time   `json:"pickup_time"`
	Items        []OrderItem `json:"items"`
	TotalPrice   int         `json:"total_price"`
	Status       string      `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
}
