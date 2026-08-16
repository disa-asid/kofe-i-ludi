-- Отзывы посетителей. approved=0 по умолчанию — модерация перед публикацией.
CREATE TABLE IF NOT EXISTS reviews (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    author_name TEXT    NOT NULL,
    rating      INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    text        TEXT    NOT NULL,
    approved    INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Курсы, которые проводит кофейня.
CREATE TABLE IF NOT EXISTS courses (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT    NOT NULL,
    description TEXT    NOT NULL,
    price       INTEGER NOT NULL,
    starts_at   DATETIME NOT NULL,
    seats_total INTEGER NOT NULL,
    seats_taken INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Записи на курс. seats_taken в courses растёт атомарно вместе с вставкой сюда.
CREATE TABLE IF NOT EXISTS course_signups (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id  INTEGER NOT NULL REFERENCES courses(id),
    name       TEXT    NOT NULL,
    phone      TEXT    NOT NULL,
    email      TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Предзаказы. items_json — простой JSON-массив позиций для демо;
-- при росте нагрузки стоит вынести в отдельную таблицу order_items.
CREATE TABLE IF NOT EXISTS orders (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    customer_name TEXT    NOT NULL,
    phone         TEXT    NOT NULL,
    pickup_time   DATETIME NOT NULL,
    items_json    TEXT    NOT NULL,
    total_price   INTEGER NOT NULL,
    status        TEXT    NOT NULL DEFAULT 'new',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reviews_approved ON reviews(approved, created_at);
CREATE INDEX IF NOT EXISTS idx_courses_starts_at ON courses(starts_at);
CREATE INDEX IF NOT EXISTS idx_signups_course ON course_signups(course_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status, created_at);
