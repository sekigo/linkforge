# LinkForge

Сервис коротких ссылок. Учебный проект. HTTP API, PostgreSQL, Redis, миграции, Docker, Kubernetes, CI.

## Стек

- Go 1.26
- PostgreSQL 16 (драйвер `pgx/v5`)
- Redis 7 (`go-redis/v9`)
- chi router
- golang-migrate (миграции БД)
- Docker compose (локальная инфра)
- GitHub Actions (CI)

## Требования

- Go 1.26+
- Docker + docker compose
- `migrate` CLI: `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

## Старт

```bash
# поднять postgres + redis
make up

# применить миграции
make migrate-up

# запустить API
make run
```


### Создать ссылку

```bash
curl -X POST http://localhost:8080/api/v1/links \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/some/long/path"}'
```

Ответ:

```json
{ "code": "1f", "short_url": "http://localhost:8080/1f", "url": "https://example.com/some/long/path" }
```

### Перейти по короткой ссылке

```bash
curl -i http://localhost:8080/1f
# 302 Found, Location: https://example.com/some/long/path
```

## Структура

```
cmd/api/            точка входа HTTP-сервиса
internal/config/    загрузка конфигурации из env
internal/domain/    доменные типы (Link)
internal/storage/   доступ к Postgres и Redis
internal/service/   бизнес-логика (генерация code, валидация URL)
internal/http/      HTTP роутер, handlers, middleware
migrations/         SQL миграции (golang-migrate)
deploy/k8s/         Kubernetes манифесты (TODO)
```
