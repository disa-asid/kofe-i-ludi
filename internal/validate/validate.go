package validate

import (
	"errors"
	"regexp"
	"strings"
)

// Валидация — это НЕ защита от SQL-инъекций (её даёт database/sql с
// плейсхолдерами, см. internal/handlers). Это защита от мусорных,
// слишком длинных или откровенно некорректных данных.

var phoneRe = regexp.MustCompile(`^\+?[0-9\s\-\(\)]{7,20}$`)
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func Trim(s string) string {
	return strings.TrimSpace(s)
}

func NotEmpty(field, value string) error {
	if Trim(value) == "" {
		return errors.New(field + " не может быть пустым")
	}
	return nil
}

func MaxLen(field, value string, max int) error {
	if len([]rune(value)) > max {
		return errors.New(field + " слишком длинный (максимум " + itoa(max) + " символов)")
	}
	return nil
}

func Phone(value string) error {
	if !phoneRe.MatchString(Trim(value)) {
		return errors.New("некорректный номер телефона")
	}
	return nil
}

func Email(value string) error {
	if value == "" {
		return nil // email опционален
	}
	if !emailRe.MatchString(Trim(value)) {
		return errors.New("некорректный email")
	}
	return nil
}

func Rating(r int) error {
	if r < 1 || r > 5 {
		return errors.New("рейтинг должен быть от 1 до 5")
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
