# Текущий контекст

Обновлено: 2026-10-09. Основание: обзор рабочего дерева и проверки перед фиксацией auth/posts/UI. Перед использованием сверяй с кодом и `git status`.

## Проект и карта

Saveboard — сохранение ссылок и заметок, планируемые архивы веб-страниц и поиск по личной коллекции.

- `backend/`: Go API, SQL-миграции, River worker, Node capture. `backend/internal/identity/` — авторизация; `notes/` — foundation; `posts/` — новая модель постов.
- `frontend/`: React SPA, общий API-клиент, UI и browser-тесты. `src/features/auth/` — сессия; `src/features/library/` — демонстрационная библиотека.
- `deploy/`, `scripts/`, `.github/workflows/ci.yaml`, `Taskfile.yml`: окружение, генерация контракта и проверки.
- Версии инструментов уточнять в `package.json`, `backend/go.mod` и lockfiles. Сейчас объявлены Node 24, pnpm 11.19.0 и Go 1.27.1.

## Наблюдаемое состояние

- Есть каркас API/worker/migrator, транзакционная запись notes + River job, BlobStore и инфраструктура Compose.
- Реализованы серверные login/refresh/logout и OIDC flow; описание — [identity](../../backend/internal/identity/README.md).
- На frontend есть login и управление сессией; [auth README](../../frontend/src/features/auth/README.md) описывает sessionStorage и refresh. Ленты и операции с постами пока демонстрационные; регистрация и OAuth UI — заглушки.
- Добавлены модель `posts` и миграция `00004_posts.sql`; существующие notes и теги продолжают существовать отдельно. Схема не доказывает готовность API/ACL.
- Capture отвечает заглушкой; полноценные архивы и ingestion/RAG не считать реализованными. Критерии развития — [P0–P5](../implementation-plan.md).
- Изменения auth/posts/UI подготовлены к фиксации по запросу пользователя вместе с миграциями, API-контрактом, тестами и документацией.

## Последние проверки

2026-10-09, из корня проекта:

- `pnpm task check` — passed: типы, lint/vet, unit-тесты, сборки и согласованность OpenAPI/TypeScript.
- `pnpm task test:integration` — passed: notes, posts, identity и HTTP API на тестовых контейнерах PostgreSQL.
- `pnpm test:e2e` с `PLAYWRIGHT_CHANNEL=chrome` — passed, 6 сценариев; ответы API подменяются в тестах.
- `git diff --check` — passed.
- `docker compose --env-file .env -f deploy/compose.yaml config --quiet` — failed: в локальном окружении отсутствует значение `AUTH_ENCRYPTION_KEY`. Полный Compose smoke в этой проверке не запускался. Подготовка недостающих ключей предусмотрена в `node scripts/init-env.mjs`.

## Продолжение

Активная продуктовая задача этим запросом не назначена. При следующем запросе создать план, если этого требует [PLANS.md](../../PLANS.md), и добавить сюда ссылку. Приоритеты P0–P5 не изменены.

Есть расхождения старых документов с текущей реализацией: [список вопросов](questions.md). Прошлые результаты в `docs/development.md` — исторические записи, а не результаты проверки текущего рабочего дерева.
