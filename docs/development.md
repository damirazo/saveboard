# Локальная разработка и границы foundation

Дата: 2026-10-07.

## Реализовано

- Один Go module: chi/Huma API, River worker, one-shot migrator, OpenAPI exporter и healthcheck binary. Env config, graceful shutdown, HTTP body/timeouts, JSON slog, OpenTelemetry HTTP instrumentation и spans. Экспортёр traces/collector пока не настроен.
- SQL-миграции users/identities/sessions, notes/text_notes, tags/note_tags и служебных receipts. Приватность по умолчанию; composite foreign keys запрещают назначение чужих тегов. Полная схема архивов/ACL/friendships добавляется будущими миграциями.
- Внутренний `notes.Service`: запись Bun + raw SQL + River enqueue в одном `sql.Tx`. Worker через pgxpool записывает идемпотентную receipt обработки `note.created`. Receipt **не** означает capture или embedding success.
- SHA-256 BlobStore: потоковая запись с лимитом, дедупликация, атомарная публикация hard link, неизменность старого содержимого, валидация ключей. Каталог доступен только доверенному worker; endpoint чтения по hash отсутствует. Lease/GC и связи manifest ещё не реализованы, `Delete` не вызывается приложением.
- React/TypeScript strict, Vite, TanStack Router/Query, openapi-fetch/react-query, генерируемый контракт. Tailwind, исходный Button в соглашениях shadcn/ui и `components.json`. React Hook Form/Zod и Tiptap установлены для следующего среза, редактора пока нет.
- Playwright, Chromium в контейнерном образе, Readability, jsdom и CSS parsers. Capture требует внутренний bearer token, но возвращает `501 capture_not_enabled`; браузер не запускается, пользовательские URL не загружаются. Сеть `internal` **не объявляется** достаточным SSRF-контролем будущего браузера.
- Keycloak realm: email verification, PKCE, confidential BFF client, Mailpit SMTP. Go discovery/verifier adapter. Полноценные login/callback, nonce/state, cookies и CSRF пока отсутствуют; `/auth/*` и notes HTTP API не включены. Тестовых пользователей и обхода авторизации нет.
- Qdrant SDK/сервис; Ollama и явная инициализация модели с digest/dimension checks. Extraction/chunking/index/ACL-check pipeline ещё отсутствует.
- CI: Go tests/vet, Testcontainers PostgreSQL, TypeScript/ESLint/Vitest/build, contract drift, browser smoke и сборка Docker images. CI запускается после помещения проекта в GitHub; сейчас Git-репозитория в каталоге нет.

## Проверки на этой машине

Выполнены: установка Go/JS dependencies; Go tests/vet; TypeScript; ESLint; Vitest; production build web/capture; OpenAPI без drift; Playwright smoke на установленном Chrome в desktop/mobile; валидация Compose с dev override и embeddings profile.

Загрузка отдельного Windows Chromium через Playwright CDN завершилась таймаутом; browser smoke выполнен на уже установленном Chrome через `PLAYWRIGHT_CHANNEL=chrome`. Linux Chromium успешно установлен в capture image; проверено наличие бинарника. Версия Playwright в package совпадает с версией закреплённого образа.

Первый запуск блокировался внутренней ошибкой Docker Desktop `initializing Inference manager` / `dockerInference`. После перезапуска пользователем движок работает; сброс Docker и удаление его данных не потребовались. Причина первоначального сбоя точно не установлена; признаков нехватки ресурсов при повторной проверке не обнаружено.

Подтверждено на реальном Docker:

- Все три прикладных image (`backend`, `web`, `capture`) собраны; внешние images скачаны по закреплённым digest.
- Все девять долгоживущих сервисов запущены; настроенные healthchecks проходят, one-shot `migrate` завершился с кодом 0.
- Testcontainers PostgreSQL: повторное применение миграций, rollback note/text/job, commit и обработка worker через отдельный pgxpool. Обычная команда `pnpm task test:integration` работает без ручного `DOCKER_HOST`.
- `pnpm task smoke`: nginx/SPA asset, readiness API, OpenAPI, отсутствие SPA fallback для неизвестного API path, корректный issuer Keycloak.
- Keycloak импортировал confidential BFF client с непустым секретом и callback `http://localhost:8080/auth/callback`.
- В capture проверены 401 без bearer token, 501 с токеном до включения архивирования, наличие bundled Chromium.
- BGE-M3 загружена: проверены digest, два RU/EN embeddings по 1024 конечных чисел и повторный init без повторной загрузки. Исправлено сравнение digest: Ollama возвращает hex без `sha256:`; тест не допускает несовпадающие или невалидные хеши.

Повторить проверки:

```sh
pnpm task test:integration
pnpm infra:up
docker compose --env-file .env -f deploy/compose.yaml ps
pnpm task smoke
pnpm task model:init
```

Транзакционная часть P0.1 подтверждена; бизнес DTO validation остаётся следующим шагом. P0.2, полноценная оценка retrieval P0.3 и продуктовый срез P1 не объявляются завершёнными. Проверка наличия Chromium не подтверждает browser sandbox, egress enforcement или качество архивов.

## API на хосте и Mailpit

```sh
docker compose --env-file .env -f deploy/compose.yaml -f deploy/compose.dev.yaml up -d postgres mailpit
```

Экспортировать `DATABASE_URL=postgres://saveboard:<APP_DB_PASSWORD>@127.0.0.1:5432/saveboard?sslmode=disable`, используя пароль из `.env`. Из корня выполнить `pnpm --dir backend migrate`, `pnpm task dev:api` и в другом терминале `pnpm task dev:worker`. В PowerShell: `$env:DATABASE_URL='postgres://...'`.

API использует порт 8080; не запускать одновременно nginx на том же порту. `pnpm dev` поднимает Vite на 5173 и проксирует API к 8080. Mailpit UI с dev override: <http://localhost:8025>, SMTP внутри Compose — `mailpit:1025`.

## Развитие

HTTP DTO/validation живут в transport; services управляют транзакциями и инвариантами; feature-specific SQL остаётся явным. Новые модули `tags`, `access`, `feed`, `archives` добавляются вместе с реализацией вместо пустых пакетов. Generic repository и ORM AutoMigrate не нужны.

После изменения API выполнить `pnpm api:generate` из корня: обновляются `backend/api/openapi.json`, его снимок `frontend/api/openapi.json` и `frontend/packages/api-client/src/schema.d.ts`. Хранить все три файла в Git. `node scripts/check-contract.mjs` проверяет drift даже без Git. Внутри frontend команда `pnpm api:generate` использует только локальный снимок. Перед подключением продуктовых handlers реализовать session/CSRF/ACL. Capture/ingestion — отдельные job kinds; недоступный Qdrant/Ollama не должен блокировать CRUD.

Прикладной код разделён на самостоятельные `backend` и `frontend` с собственными pnpm workspaces/locks, TypeScript/ESLint, Dockerfile и README. Capture и embedding init относятся к backend. Общие Compose, Task, CI, bootstrap, проверка live-stack и синхронизация контракта остаются в корне/`deploy`. Нет `workspace:` зависимостей между backend и frontend. `tools` содержит только общий task runner, он не нужен для самостоятельных `pnpm check` внутри частей.

## Локальное окружение

Compose предназначен для разработки: Keycloak `start-dev`, HTTP/SMTP без TLS, nginx только на loopback. Production требует TLS, secret management, корректного issuer routing, archive origin, проверенного browser sandbox/egress, backup/restore и наблюдаемости.

Issuer после старта: `http://localhost:8080/identity/realms/saveboard` (порт из `.env`). При реализации BFF обеспечить разрешение **того же issuer** из API-контейнера через hostname/edge routing; нельзя подменять issuer внутренним URL и отключать verification.

Внешние image digests проверены через Docker Hub/MCR/Quay. Основания настройки: [Compose startup order](https://docs.docker.com/compose/how-tos/startup-order/), [Playwright Docker](https://playwright.dev/docs/docker), [River database/sql](https://pkg.go.dev/github.com/riverqueue/river/riverdriver/riverdatabasesql).

`embedding-model.json` фиксирует BGE-M3 digest. Init сверяет upstream manifest **до** загрузки и локальный digest **после**, затем использует имя с hash. При изменении upstream tag init останавливается. Загрузка около 1.2 GB; проверка двух embeddings не заменяет RU/EN evaluation и оценку RAM.

Volumes: PostgreSQL включая Keycloak DB, CAS, Qdrant, Ollama. `down` сохраняет их. Смена `.env` не меняет уже созданные DB roles/realm: ротацию выполнять средствами сервиса. Init SQL/realm применяется при первом создании. Не использовать `down -v` для обычного перезапуска.
