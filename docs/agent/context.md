# Текущий контекст

Обновлено: 2026-10-14. Основание: фоновая индексация текста ссылок для RAG. Перед использованием сверяй с кодом и `git status`.

## Проект и карта

Saveboard — сохранение ссылок и заметок, планируемые архивы веб-страниц и поиск по личной коллекции.

- `backend/`: Go API, SQL-миграции, River worker, Node capture. `backend/internal/identity/` — авторизация; `notes/` — foundation; `posts/` — новая модель постов.
- `frontend/`: React SPA, общий API-клиент, UI и browser-тесты. `src/features/auth/` — сессия; `src/features/library/` — демонстрационная библиотека.
- `deploy/`, `scripts/`, `.github/workflows/ci.yaml`, `Taskfile.yml`: окружение, генерация контракта и проверки.
- Версии инструментов уточнять в `package.json`, `backend/go.mod` и lockfiles. Сейчас объявлены Node 24, pnpm 11.19.0 и Go 1.27.1.

## Наблюдаемое состояние

- Есть каркас API/worker/migrator, транзакционная запись notes + River job, BlobStore и инфраструктура Compose.
- Реализованы серверные login/refresh/logout и OIDC flow; описание — [identity](../../backend/internal/identity/README.md).
- На frontend есть login/register и управление сессией; [auth README](../../frontend/src/features/auth/README.md) описывает sessionStorage и refresh. Регистрация создаёт активного пользователя с непроверенным email и сессию. Ленты и CRUD подключены к API; только OAuth UI остаётся заглушкой.
- Posts API поддерживает заметки Markdown, ссылки, фото, теги и перевод в draft. Миграция `00005_post_management.sql` обязательна. Права проверяются по активной сессии и владельцу; public/private, без friends/unlisted в форме. Legacy notes остаётся отдельным. Детали: [ADR-0001](../decisions/0001-post-content-and-uploads.md).
- Для link-постов реализованы фоновые статические копии и PNG-эскизы: River, Chromium, egress-шлюз, immutable CAS, opaque sandbox просмотра. Обязательна миграция `00006_link_captures.sql`. Границы — [ADR-0002](../decisions/0002-static-link-captures.md). Интерактивные зеркала не реализованы; раздел «Архив страниц» отдельно ещё не подключён.
- `index_rag=false` включает отдельную индексацию link: Chromium/Readability → canonical document PostgreSQL → chunks → locked Ollama BGE-M3 → Qdrant. Нужна `00008_link_rag.sql`. Срок 30 минут, попытка 5 минут, три попытки, уведомления и очистка поколений. Capture/RAG делят один слот; общий rescue теперь 6 минут. Поискового/RAG-chat endpoint пока нет. [ADR-0004](../decisions/0004-link-rag-indexing.md), [search README](../../backend/internal/search/README.md).
- Очередь capture имеет три попытки, один слот, таймаут попытки 2 минуты и общий срок 15 минут, восстановление и очистку терминальных задач. Уведомления с персональным прочтением, пагинацией и важностью требуют миграции `00007_notifications.sql`; исход обработки атомарно уведомляет владельца. Детали: [ADR-0003](../decisions/0003-durable-capture-notifications.md), [jobs](../../backend/internal/jobs/README.md), [notifications](../../backend/internal/notifications/README.md).
- По запросу пользователя от 2026-10-14 изменения авторизации, копий ссылок, очереди, уведомлений и RAG-индексации включены в общий коммит вместе с миграциями, API-контрактом, тестами и документацией.

## Последние проверки

2026-10-09, из корня проекта:

- `pnpm task check` — passed: типы, lint/vet, unit-тесты, сборки и согласованность OpenAPI/TypeScript.
- `pnpm task test:integration` — passed: notes, posts, identity и HTTP API на тестовых контейнерах PostgreSQL.
- `pnpm test:e2e` с `PLAYWRIGHT_CHANNEL=chrome` — passed, 6 сценариев; ответы API подменяются в тестах.
- `git diff --check` — passed.
- `docker compose --env-file .env -f deploy/compose.yaml config --quiet` — failed: в локальном окружении отсутствует значение `AUTH_ENCRYPTION_KEY`. Полный Compose smoke в этой проверке не запускался. Подготовка недостающих ключей предусмотрена в `node scripts/init-env.mjs`.

## Продолжение

Завершено и выложено локально: [управление постами](tasks/2026-10-09-post-management.md). API/worker/frontend пересобраны, миграции применены, приложение на `http://localhost:8080`; live Compose smoke прошёл. Недостающие auth-ключи добавлены в локальный `.env`, прежнее ограничение Compose снято. Результаты проверки этого изменения фиксируются в плане задачи; записи выше относятся к предыдущему коммиту. Приоритеты P0–P5 не изменены.

Есть расхождения старых документов с текущей реализацией: [список вопросов](questions.md). Прошлые результаты в `docs/development.md` — исторические записи, а не результаты проверки текущего рабочего дерева.

Формы входа/регистрации реализованы и обновлены на 8080: [задача auth-forms](tasks/2026-10-09-auth-forms.md). Проверен настоящий browser → API → PostgreSQL цикл регистрации/выхода/входа; временный аккаунт удалён. Реализация включена в общий коммит от 2026-10-14.

Фоновые копии ссылок: [план и результаты](tasks/2026-10-13-link-capture.md). Запущены API/worker/capture/capture-egress/frontend, volume blobs читается API только read-only, отдельный capture-cache хранит общие CSS. До ready копия и эскиз отсутствуют; старые ревизии не публикуются. Проверен живой browser → API → River → Chromium → CAS → приватный viewer. Реализация включена в общий коммит от 2026-10-14.

Очередь и уведомления: [план и проверки 2026-10-14](tasks/2026-10-14-capture-notifications.md). Реализованы `/notifications`, колокольчик и `/posts/$postId`. На 8080 обновлены API/worker/frontend и миграция. Проверены реальный River после остановки, очистка/дедупликация, ACL, 12 e2e-сценариев и живой цикл создания/прочтения уведомления. Ограничение конкурентности относится к одному процессу; текущий Compose использует один capture worker. Реализация включена в общий коммит от 2026-10-14.

RAG-индексация: [план и результаты](tasks/2026-10-14-link-rag.md). Локально обновлены API/worker/capture/frontend. Прошли общий check, все integration, 13 e2e, browser SSRF smoke и настоящий browser → River → Readability → Ollama → Qdrant цикл с уведомлением и удалением точек после снятия флага. Закреплённая BGE-M3 уже установлена, скачивать заново не потребовалось. Реализация включена в общий коммит от 2026-10-14.

Поиск по постам обновлён 2026-10-15: подстрока в названии/Markdown и отдельный режим обязательных/исключённых тегов, серверная фильтрация до пагинации. Границы public/mine сохранены; справочник тегов доступен гостям. [План и проверки](tasks/2026-10-15-post-search.md). Compose с новой логикой поиска ещё не пересобран.
