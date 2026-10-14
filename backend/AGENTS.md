# Backend

Применяется вместе с корневым `AGENTS.md` ко всему `backend/`, включая capture.

- Go module: API на chi/Huma, Bun/PostgreSQL, River worker. Capture — отдельный Node/TypeScript сервис в `capture/`.
- HTTP DTO и валидация — `internal/transport/httpapi`; бизнес-инварианты и транзакции — сервисы; SQL оставляй явным. Не добавляй generic repository или ORM AutoMigrate.
- Изменение схемы оформляй новой SQL-миграцией в `migrations/`. Не переписывай уже применённую миграцию. Проверяй совместимость существующих данных и миграции с нуля.
- Изменение сущности и постановка связанного River job должны оставаться в одной транзакции. Повтор jobs не должен дублировать результат.
- `notes` и `posts` сейчас сосуществуют. Их теги имеют разную модель владения; не объединяй таблицы без явного решения и плана миграции. См. [posts README](internal/posts/README.md).
- Для auth прочитай [identity README](internal/identity/README.md). Защищённый endpoint проверяет активность пользователя и сессии, а не только подпись JWT. Не связывай OAuth-аккаунты по совпавшему email.
- Поле visibility не является проверкой доступа. При добавлении выдачи постов/файлов/поиска проверяй текущие права на сервере до выдачи данных, включая counts и snippets.
- Capture выполняет статические снимки через изолированный egress-шлюз; см. [capture README](capture/README.md) и ADR-0002. Изменения загрузки URL требуют проверки фактического egress/SSRF и opaque sandbox просмотра. Сеть Compose `internal` не заменяет такую проверку; browser-smoke обязателен при изменении границы.
- Недоступность capture/Qdrant/Ollama не должна блокировать базовый CRUD; результат receipt job не равен успешному архиву или индексации.

## Проверки из корня

- Go: `go -C backend test ./...` и `go -C backend vet ./...`; форматируй изменённые Go-файлы через `gofmt`.
- Вся backend-часть с capture: `pnpm --dir backend check`.
- SQL, транзакции, auth и модели: `pnpm --dir backend test:integration` (требует Docker; создаёт тестовые контейнеры).
- API: `pnpm api:generate`, затем `node scripts/check-contract.mjs` и `pnpm --dir frontend typecheck`.
- При изменении образа capture согласуй версии Playwright и Chromium; проверка сборки не доказывает безопасность браузера.
