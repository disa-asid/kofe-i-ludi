# Cafe Backend

Бекенд на Go для сайта кофейни: отзывы посетителей, предзаказ на самовывоз,
запись на курсы. SQLite, без внешнего фреймворка — только стандартная
библиотека `net/http` (Go 1.22+ роутинг с методами и `{id}` в пути) плюс
драйвер SQLite.

## Запуск

Нужен только Go 1.22+. Компилятор C НЕ требуется — драйвер SQLite
(`modernc.org/sqlite`) написан на чистом Go, работает одинаково на
Windows/Mac/Linux без cgo.

```bash
cd cafe-backend
go get modernc.org/sqlite@latest
go mod tidy
```

Запуск (Mac/Linux):
```bash
ADMIN_TOKEN=любой-секретный-токен go run ./cmd/server
```

Запуск (Windows, cmd.exe):
```cmd
set ADMIN_TOKEN=любой-секретный-токен
go run ./cmd/server
```

Запуск (Windows, PowerShell):
```powershell
$env:ADMIN_TOKEN="любой-секретный-токен"
go run ./cmd/server
```

Сервер поднимется на `:8080`. Переменные окружения:

| переменная       | по умолчанию              | что делает                          |
|------------------|----------------------------|--------------------------------------|
| `PORT`           | `8080`                     | порт сервера                         |
| `DB_PATH`        | `./cafe.db`                | путь к файлу SQLite                  |
| `ADMIN_TOKEN`    | *(пусто → админка выключена)* | токен для `/api/admin/*`         |
| `ALLOWED_ORIGIN` | `http://localhost:5173`    | домен фронтенда, для CORS            |

## API

### Публичные

```
GET  /api/reviews?limit=20&offset=0        — список одобренных отзывов
POST /api/reviews                          — оставить отзыв (на модерации)
     {"author_name": "...", "rating": 1-5, "text": "..."}

GET  /api/courses                          — список будущих курсов + свободные места
POST /api/courses/{id}/signup              — запись на курс
     {"name": "...", "phone": "+7...", "email": "необязательно"}

POST /api/orders                           — предзаказ
     {"customer_name": "...", "phone": "+7...", "pickup_time": "RFC3339",
      "items": [{"name": "Латте", "qty": 2, "price": 210}]}
GET  /api/orders/{id}?phone=+7...          — статус своего заказа
```

### Админские (заголовок `Authorization: Bearer <ADMIN_TOKEN>`)

```
GET    /api/admin/reviews?status=pending
PATCH  /api/admin/reviews/{id}/approve
DELETE /api/admin/reviews/{id}

POST   /api/admin/courses
GET    /api/admin/courses/{id}/signups

GET    /api/admin/orders?status=new
PATCH  /api/admin/orders/{id}/status       {"status": "confirmed"}
```

## Примеры curl

```bash
# оставить отзыв
curl -X POST localhost:8080/api/reviews \
  -d '{"author_name":"Аня","rating":5,"text":"Очень уютно"}'

# одобрить его (узнать id из GET /api/admin/reviews)
curl -X PATCH localhost:8080/api/admin/reviews/1/approve \
  -H "Authorization: Bearer <ADMIN_TOKEN>"

# записаться на курс
curl -X POST localhost:8080/api/courses/1/signup \
  -d '{"name":"Иван","phone":"+79991234567"}'

# сделать предзаказ
curl -X POST localhost:8080/api/orders \
  -d '{"customer_name":"Мария","phone":"+79995554433",
       "pickup_time":"2026-09-01T12:00:00Z",
       "items":[{"name":"Латте","qty":2,"price":210}]}'
```

## Безопасность — что уже сделано и почему

**SQL-инъекции.** Все запросы к БД идут через параметризованные запросы
(`?`-плейсхолдеры в `database/sql`) — значения никогда не подставляются
в текст SQL строкой. Драйвер сам экранирует данные на уровне протокола,
поэтому `"; DROP TABLE reviews;--"` в поле имени просто ляжет в базу как
текст. Проверено вручную (см. коммит-тест ниже) — таблица после такой
попытки цела.

**Гонки при записи на курс (overbooking).** Проверка и уменьшение
свободных мест происходит одним атомарным `UPDATE ... WHERE seats_taken <
seats_total` внутри транзакции — если два человека одновременно жмут
"записаться" на последнее место, второй гарантированно получит `409`,
а не перезапишет счётчик поверх первого.

**Rate limiting.** Простой token-bucket на IP без внешних зависимостей:
10 запросов/минуту на создание (отзывы, заказы, записи), 120/минуту на
чтение. Защищает от спама форм и грубого перебора.

**CORS.** Разрешён только домен из `ALLOWED_ORIGIN` — с других сайтов
API дёрнуть через браузер не выйдет.

**Админка.** Простой bearer-токен, сравнение через
`subtle.ConstantTimeCompare` (защита от timing-атак при переборе токена).
Если `ADMIN_TOKEN` не задан — админ-эндпоинты отключены целиком
(502, а не "открыты по умолчанию").

**Приватность заказов.** `GET /api/orders/{id}` требует ещё и `phone` —
нельзя просто перебрать id и посмотреть чужие заказы.

**Лимиты на размер и объём данных.** Тело запроса ограничено 1 МБ
(`http.MaxBytesReader`), длина текстовых полей — вручную (имя ≤ 80
символов, отзыв ≤ 2000, и т.д.), максимум 30 позиций в заказе.

**Таймауты сервера.** `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`
выставлены явно — защита от slowloris-атак (когда клиент держит
соединение открытым и не шлёт данные).

## Чего здесь нет и что стоит добавить перед продакшеном

- **HTTPS** — сервер сам его не поднимает. Ставится перед ним Caddy
  (сам получает сертификат Let's Encrypt) или nginx + certbot.
- **Полноценная авторизация админки** — сейчас один статичный токен на
  всех. Для нескольких сотрудников кофейни нужны нормальные логин/пароль
  с bcrypt и, возможно, сессии или JWT.
- **Уведомления** — сейчас заказы/записи просто падают в БД, никто не
  узнает о новом заказе, пока сам не зайдёт в админку. Стоит добавить
  отправку в Telegram-бота владельцу (у тебя уже есть такой опыт с
  network_scanner.py).
- **Резервное копирование БД** — SQLite-файл стоит бэкапить по крону
  (простой `cp cafe.db backups/cafe-$(date +%F).db`).
