package db

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Open открывает SQLite-базу и применяет схему.
// Файл БД создаётся автоматически, если его ещё нет.
func Open(path string) (*sql.DB, error) {
	// modernc.org/sqlite — чистый Go-драйвер без cgo, поэтому работает
	// на Windows/Mac/Linux без установки C-компилятора.
	// _pragma=foreign_keys(1) включает проверку внешних ключей,
	// journal_mode(wal) — разрешает параллельное чтение во время записи.
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(wal)", path)

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// SQLite не любит параллельную запись из нескольких горутин через
	// один и тот же файл — ограничиваем пул одним соединением на запись.
	// Для чтения это не мешает, WAL режим разрешает параллельное чтение.
	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return conn, nil
}
