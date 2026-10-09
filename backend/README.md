# Saveboard backend

Самостоятельная часть проекта: Go API, River worker, migrator, OpenAPI exporter, Node/Playwright capture-service и конфигурация embeddings. Можно перенести весь каталог в отдельный репозиторий: исходники и зависимости фронтенда здесь не используются.

Требования: Go 1.27.1, Node.js 24, pnpm 11.19.0. Из этого каталога:

```sh
go mod download
pnpm install --frozen-lockfile
pnpm check
pnpm test:integration
pnpm api:generate
```

`pnpm check` проверяет Go и capture. Интеграционный тест использует Docker/Testcontainers и отдельную временную БД. `pnpm api:generate` экспортирует `api/openapi.json`, не обращаясь к frontend.

Для запуска задать `DATABASE_URL`, `AUTH_JWT_KEY` и `AUTH_ENCRYPTION_KEY` (разные случайные ключи по 32 байта, hex), затем `pnpm migrate`, `pnpm dev`, в другом терминале `pnpm dev:worker`. Capture отдельно: `pnpm dev:capture` с `CAPTURE_TOKEN` длиной от 32 символов. API входа по username/email, JWT/refresh, OAuth/OIDC и выхода описан в [identity/README.md](internal/identity/README.md). Capture пока не включён.

Docker build-контекст — этот каталог:

```sh
docker build -t saveboard/backend:dev .
docker build -f capture/Dockerfile -t saveboard/capture:dev .
```

API image также содержит `/app/worker`, `/app/migrate`, `/app/openapi`, `/app/healthcheck`. Для запуска нужны PostgreSQL и сервисные настройки; монорепозиторий предоставляет их через общий `deploy`.

`scripts/init-model.mjs` читает `config/embedding-model.json`; `OLLAMA_URL` задаёт адрес runtime. Он не зависит от путей frontend или общего deploy. Скрипт скачивает кандидат BGE-M3 и проверяет digest/embeddings, не реализуя полный ingestion.

Граница интеграции с фронтендом — HTTP/OpenAPI. После разделения репозиториев публиковать `api/openapi.json` версионированным CI-артефактом; импорт Go-кода в frontend не требуется. Имя Go module `saveboard` сохранено, поэтому перенос не меняет внутренние import paths.
