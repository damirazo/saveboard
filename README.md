# Saveboard

Saveboard — проект приложения для сохранения ссылок и заметок, архивирования веб-страниц и поиска по личной коллекции. Основной стек: Go, React/TypeScript и PostgreSQL; локальное окружение запускается через Docker Compose.

Базовый каркас монорепозитория по сохранённому ресерчу: Go API/worker, React SPA, OpenAPI-клиент, миграции и локальное окружение Docker Compose. Это **foundation**, ещё не готовый продукт. Backend API входа по username/email, JWT с ротацией refresh, OAuth/OIDC и управления сессиями реализован; интерфейс входа, CRUD заметок, архивирование и поиск — следующие этапы. Настройка авторизации: [identity README](backend/internal/identity/README.md). Фактический объём и результаты проверок: [development.md](docs/development.md).

## Быстрый запуск

Нужны Docker с Linux-контейнерами и Compose, Node.js 24, pnpm 11.19.0. Для локальной Go-разработки — Go 1.27.1 (при `GOTOOLCHAIN=auto` загружается автоматически). Выполнять из корня проекта:

```sh
npm install -g pnpm@11.19.0
pnpm run bootstrap
pnpm infra:up
```

`init-env` создаёт `.env` со случайными паролями; для существующего файла добавляет недостающие auth-ключи, сохраняя текущие значения. Секреты исключены из Git.

- Приложение: <http://localhost:8080>.
- API: <http://localhost:8080/api/docs> и <http://localhost:8080/api/openapi.json>.
- Keycloak: <http://localhost:8080/identity/admin/>; admin credentials в `.env`.
- Состояние: `docker compose --env-file .env -f deploy/compose.yaml ps`.
- Логи: `pnpm infra:logs`; остановка с сохранением данных: `pnpm infra:down`.

Compose включает `postgres`, `migrate`, `api`, `worker`, `capture`, `keycloak`, `qdrant`, `ollama`, `mailpit`, `nginx`. Наружу открыт только nginx на loopback. Goose/River миграции выполняет один `migrate` перед API/worker; Keycloak использует отдельную БД и роль. Внешние images закреплены по digest.

Ollama-модель загружается отдельно: `pnpm task model:init` (около 1.2 GB). Команда сверяет digest BGE-M3 и размерность embeddings 1024. Оценка качества поиска остаётся P0.3.

## Разработка и проверки

```sh
pnpm dev                 # Vite :5173, API proxy → localhost:8080
pnpm api:generate        # Go DTO → OpenAPI → TypeScript
pnpm task check          # tests/vet, tsc, lint, Vitest, build, contract drift
pnpm task test:integration
pnpm task smoke          # HTTP-проверки запущенного Compose
pnpm --dir frontend exec playwright install chromium
pnpm test:e2e
```

Go Task закреплён в отдельном модуле `tools`; глобальная установка не нужна. Browser smoke использует API fixtures; PostgreSQL/River проверяет отдельный Testcontainers-тест. Для установленного Chrome в PowerShell: `$env:PLAYWRIGHT_CHANNEL='chrome'; pnpm test:e2e`.

После перезапуска Docker все сервисы Compose запущены. Подтверждены PostgreSQL/River integration test, HTTP smoke, healthchecks, установка Chromium в capture и BGE-M3 embeddings с проверкой digest/размерности. Локальные сборки и unit/browser tests также пройдены. Подробный статус и ограничения — [development.md](docs/development.md).

## Структура

```text
backend/                самостоятельный Go + Node workspace
  cmd/ internal/        API, worker, migrator и прикладной код
  migrations/           SQL Goose, встроенные в бинарник
  capture/              Node/Playwright service и его Dockerfile
  api/openapi.json      контракт, экспортированный Go
  scripts/ config/      модель embeddings и её инициализация
  go.mod go.sum         Go dependencies
  pnpm-lock.yaml        отдельный lock JS-зависимостей бэкенда
  Dockerfile            сборка Go-процессов, context: backend
frontend/               самостоятельный React/pnpm workspace
  src/ e2e/             UI и браузерные проверки
  packages/api-client/  generated types + openapi-fetch/query
  api/openapi.json      входной снимок контракта бэкенда
  pnpm-lock.yaml        отдельный lock зависимостей фронтенда
  Dockerfile nginx.conf сборка и отдача SPA, context: frontend
deploy/                 Compose, Keycloak, PostgreSQL
scripts/                общие bootstrap/contract/smoke сценарии
tools/                  общий Go Task
docs/                   ресерч и межсервисная архитектура
```

В корне нет общего JS workspace и прикладных зависимостей. `package.json` и `Taskfile.yml` только координируют две части. У каждой части собственные конфигурации TypeScript/ESLint и lock-файлы; Docker не копирует исходники соседней части.

При выделении в репозитории переносится каталог целиком: [backend/README.md](backend/README.md), [frontend/README.md](frontend/README.md). Фронтенд компилируется без Go и исходников бэкенда. `pnpm api:generate` в корне экспортирует backend-контракт, копирует его в frontend и обновляет клиент; после разделения вместо копирования можно доставлять этот же файл артефактом CI.

## Начать отсюда

1. [Выбор инструментов и сравнение альтернатив](docs/research.md).
2. [Спецификация архитектуры и поведения](docs/architecture.md).
3. [Этапы реализации и критерии приемки](docs/implementation-plan.md).
4. [Машиночитаемый состав стека](docs/stack.json).
5. [Пример SQL: ALL / ANY / NONE + права доступа](backend/docs/examples/search-notes.sql).

Для следующего этапа: завершить проверочные прототипы P0, затем реализовать вертикальный срез P1. `stack.json` — спецификация; фактические версии закреплены в `backend/go.mod`/`go.sum`, отдельных `backend/pnpm-lock.yaml`, `frontend/pnpm-lock.yaml` и Compose/Dockerfiles. Схемы из ресерча — проектные примеры; миграции находятся в `backend/migrations`.

## Рекомендуемый набор

- Backend: Go, chi, Huma v2, Bun ORM, pgx v5 через `database/sql`, Goose, River.
- Данные: PostgreSQL, локальное файловое хранилище с дедупликацией по SHA-256, Qdrant.
- Авторизация: Keycloak, OIDC Authorization Code + PKCE, серверные сессии Go в PostgreSQL.
- Frontend: React, TypeScript, Vite, TanStack Router / Query, openapi-typescript / openapi-fetch, shadcn/ui, Tailwind CSS, React Hook Form / Zod, Tiptap.
- Сохранение страниц: отдельный Node.js / Playwright / Chromium capture-service, Mozilla Readability, собственный manifest и слой путей; Browsertrix / ReplayWeb.page — кандидат для сложного воспроизведения JavaScript после отдельного прототипа.
- Векторизация: Go worker → локальный Ollama с BGE-M3 → Qdrant; модель предварительная до проверки на русских и английских документах.
- Эксплуатация: Docker Compose, nginx, Go Task, pnpm workspaces, OpenTelemetry и структурированные логи.

Главная оговорка: универсальное «полное статическое зеркало любого сайта со всей работоспособностью JavaScript» недостижимо. Сохраняем видимое состояние и ресурсы страницы, явно показываем неполное сохранение; интерактивное архивное воспроизведение — отдельный режим. Пути ресурсов и физическая дедупликация решаются раздельно, чтобы новые версии файлов не портили старые снимки.
