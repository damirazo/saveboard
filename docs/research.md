# Выбор стека Saveboard

Дата проверки: 2026-10-07. Статус: рекомендуемая исходная архитектура, до проверки интеграционными прототипами. «Выбрано» означает проектную рекомендацию, а не результат измерений производительности.

## 1. Критерии и допущения

Приоритеты: Go + PostgreSQL; ORM и обычный SQL в одной транзакции; SQL-миграции; структуры входа/выхода REST API и валидация; точный поиск ALL / ANY / NONE по тегам; отсутствие утечек приватных заметок; воспроизводимые архивы; последующее использование Qdrant для RAG; монорепозиторий и Compose.

Исходные допущения: один сервер Linux / Docker, одна команда разработки, без обязательного SSR/SEO, без совместного редактирования в реальном времени. На Windows — Docker Desktop с Linux-контейнерами. Масштаб и бюджет не заданы: здесь нет утверждения о конкретном RPS, стоимости или достаточности конкретного объема RAM. Самые тяжелые процессы — Chromium и embedding-модель; их параллелизм регулируется независимо от API.

Пользовательские заметки приватны по умолчанию. «Друзья» — подтвержденная двусторонняя связь. Email-вход первоначально означает email + пароль и подтверждение адреса. Magic link, OAuth-провайдеры, размеры файлов и SEO остаются продуктовыми параметрами; они не блокируют выбор ядра.

## 2. Go API: chi + Huma v2

**Выбор:** `net/http`, `github.com/go-chi/chi/v5`, `github.com/danielgtaylor/huma/v2`, адаптер `humachi`.

Huma связывает типизированные входные/выходные структуры с OpenAPI и JSON Schema и выполняет входную валидацию. Это наиболее прямое соответствие требованию пользователя. chi дает привычный HTTP-router и middleware. Документация: [Huma](https://github.com/danielgtaylor/huma), [валидация](https://huma.rocks/features/request-validation/).

В HTTP-слое отдельные DTO; Bun-модели не становятся API-контрактом. Структурная валидация — Huma; проверки вида «эта ссылка доступна для скачивания», «теги принадлежат пользователю», «этот subtype соответствует kind» — сервисный слой. Не подключать второй валидатор Go для дублирования тех же тегов. OpenAPI описывает ответы, но корректность исходящих данных и отсутствие секретных полей проверяются контрактными тестами.

Альтернативы:

- Gin / Echo с дополнительным генератором OpenAPI работоспособны, но для этой задачи требуют больше согласования документации и валидаторов. Они оправданы, если команда уже использует их.
- Стандартный `http.ServeMux` + Huma еще компактнее; chi выбран за удобство группировки и middleware. При желании уменьшить зависимости можно заменить router, сохранив Huma.
- Goa полезен при предпочтении DSL и генерации сервисного кода. Здесь исходным контрактом должны быть Go-структуры, поэтому дополнительный DSL не дает явной выгоды.
- Fiber не дает необходимого здесь преимущества перед стандартным HTTP-стеком; выбор не основан на синтетическом рейтинге скорости.

## 3. PostgreSQL: Bun + pgx + Goose

**Выбор ORM:** Go-пакет `github.com/uptrace/bun`, не JavaScript runtime Bun. Он построен поверх `database/sql`, поддерживает модели, связи, SQL-oriented query builder и прямой SQL. Драйвер: `github.com/jackc/pgx/v5/stdlib`; диалект: `github.com/uptrace/bun/dialect/pgdialect`. [Bun](https://bun.uptrace.dev/guide/), [транзакции](https://bun.uptrace.dev/guide/transactions.html), [pgx](https://github.com/jackc/pgx/).

CRUD и несложные выборки — Bun. Поиск тегов, лента, сложные CTE — параметризованный SQL через тот же pool и ту же транзакцию. `bun.Tx` оборачивает `*sql.Tx`. Нельзя начать транзакцию через Bun, а часть ее операций случайно выполнить через глобальный `db` или другой pgx pool.

Placeholder-правило: Bun builder / `NewRaw` используют синтаксис Bun (`?`); `database/sql` с pgx — PostgreSQL `$1`, `$2`. Пример в `backend/docs/examples/search-notes.sql` предназначен именно для второго пути. Идентификаторы сортировки нельзя передавать как значения: использовать whitelist.

Альтернативы:

- **GORM** поддерживает raw SQL; он не исключается по этому признаку. Bun предпочтительнее для проекта, где сложный SQL — основная часть продукта. [GORM SQL builder](https://gorm.io/docs/sql_builder.html).
- **Ent** дает генерацию типизированной модели и SQL-интеграцию; хороший вариант при приоритете compile-time API модели. Здесь выбран меньший объем генерации. [Ent SQL integration](https://entgo.io/docs/sql-integration/).
- **sqlc + pgx** — сильный выбор для SQL-first без ORM, но sqlc сам по себе не ORM. Поэтому это альтернатива при пересмотре требования, а не основной стек. Не добавлять sqlc вместе с Bun без конкретной необходимости. [sqlc](https://docs.sqlc.dev/en/latest/).

**Миграции:** `github.com/pressly/goose/v3`, преимущественно SQL Up / Down. Создание файла миграции поддерживается, но автоматический schema diff из Go-моделей не является выбранным процессом. SQL пишется и ревьюится явно. При реальном требовании к автогенерации diff отдельно оценить Atlas и условия нужных функций. В production не использовать ORM AutoMigrate. [Goose](https://pressly.github.io/goose/).

Goose — единственный механизм миграций прикладной схемы; миграции River выполняются штатным migrator River из того же one-shot процесса, но с собственным учетом версий. Миграции Keycloak остаются под управлением Keycloak в отдельной БД. Не запускать несколько конкурентных migrator-контейнеров.

**PostgreSQL 18** — базовая major-линия для нового проекта; фиксировать актуальный patch при создании lock-набора. Major 17 допустим, если инфраструктура его уже использует. Поддержку сверять с [официальной политикой PostgreSQL](https://www.postgresql.org/support/versioning/).

## 4. Поиск тегов и текста

Теги — реляционные `tags` + `note_tags`, без массива тегов как единственного источника истины. Индексы `(note_id, tag_id)` и `(tag_id, note_id)`. Семантика `all`, `any`, `none`; полноценный AST логических выражений можно добавить позже.

PostgreSQL покрывает точную фильтрацию, связи друзей, сортировку, полнотекстовый индекс `tsvector` / GIN. Для подсказок тегов при необходимости `pg_trgm`. Это архитектурный выбор: отдельный Elasticsearch / OpenSearch / Meilisearch на старте усложнил бы согласованность без доказанной потребности. Qdrant не заменяет транзакционную проверку доступа.

Планировать нагрузочный тест на неоднородных тегах: несколько очень популярных, много редких, разные доли приватных записей. Сравнить `EXISTS` / `NOT EXISTS` с `GROUP BY ... HAVING` и стартом от наиболее селективного тега. Индексы и план определяются через `EXPLAIN (ANALYZE, BUFFERS)`, а не предположением о всегда лучшем SQL.

## 5. Фоновые задачи: River

**Выбор:** River поверх того же PostgreSQL. Сохранение note и постановка capture-job выполняются в одной транзакции. Это убирает разрыв «заметка создана, задача потеряна» без Redis. [Интеграция River с Bun](https://riverqueue.com/blog/database-sql).

API вставляет задачи через `riverdatabasesql` и `bunTx.Tx`. Go worker исполняет их через `riverpgxv5` и собственный pgxpool. Это намеренно разные процессы; общий атомарный commit нужен только при постановке задачи. Для единого SQL-driver в worker существует `NewWithPgxListener`; старое утверждение, что `database/sql` всегда означает только polling, уже неполно. [Текущий API драйвера](https://pkg.go.dev/github.com/riverqueue/river/riverdriver/riverdatabasesql).

Доставка рассматривается как at-least-once. Уникальность job не заменяет идемпотентность внешних эффектов. Повторы, ограничение параллелизма, таймауты и управление ошибками нужны на каждой стадии. Цепочка capture → extract → embed → index хранится как состояние приложения и отдельные задачи. Не закладывать зависимость от платных River Pro workflows.

Asynq потребовал бы Redis и решения атомарности с PostgreSQL; Temporal оправдан при существенно более сложных длительных процессах. В начальный стек они не входят.

## 6. Авторизация: Keycloak + Go BFF

**Выбор:** Keycloak управляет регистрацией, паролями, email verification/recovery, подключением внешних identity providers. Go использует `github.com/coreos/go-oidc/v3/oidc` и `golang.org/x/oauth2`. Браузер получает только сессионную cookie; токены провайдера остаются на сервере. [Keycloak OIDC](https://www.keycloak.org/securing-apps/oidc-layers), [администрирование](https://www.keycloak.org/docs/26.8.0/server_admin/), [go-oidc](https://github.com/coreos/go-oidc/blob/v3/README.md).

Цена выбора — дополнительный Java-сервис и обслуживание его обновлений, SMTP и настроек realm. Преимущество — не писать собственный жизненный цикл паролей и восстановления. Для небольшого сервиса, где важнее минимум контейнеров, допустим Go-native auth с `oauth2`, `go-oidc`, Argon2id и серверными сессиями; это заметный объем собственной security-sensitive логики, а не бесплатное упрощение. Управляемый OIDC-провайдер может заменить Keycloak, если допустима внешняя зависимость.

Права на заметки и дружбу остаются в PostgreSQL приложения. Не создавать Keycloak-role на каждую заметку и не помещать весь список друзей в JWT. Идентичность — `(issuer, subject)`, не один email. Автоматическое объединение учетных записей по совпавшему непроверенному email запрещено.

## 7. Архивирование: самая сложная часть

### Сравнение подходов

- **GNU Wget:** сохраняет связанные HTML/CSS-ресурсы, иерархию и переписывает ссылки; хорош для статических страниц. Не выполняет браузерный JavaScript, поэтому не закрывает современный web. [Wget manual](https://www.gnu.org/software/wget/manual/wget.html).
- **SingleFile:** удобный автономный HTML, но модель единого файла не соответствует общей внешней библиотеке ресурсов с исходными путями. Подходит как дополнительный экспорт. [SingleFile](https://github.com/gildas-lormeau/SingleFile/blob/master/README.MD?plain=1).
- **Browsertrix Crawler + ReplayWeb.page:** специализированный архивный формат и replay лучше соответствуют сложным динамическим страницам. WACZ не является обычным каталогом статических файлов и сам по себе не решает требуемую межстраничную дедупликацию. Кандидат на второй режим после прототипа. [Browsertrix](https://crawler.docs.browsertrix.com/), [QA replay](https://crawler.docs.browsertrix.com/user-guide/qa/).
- **Playwright + Chromium + собственный mirror pipeline:** выбран для контролируемых снимков с manifest, путями и CAS. Playwright дает браузер и доступ к сетевым ответам, но не готовое надежное зеркало. Основная собственная работа — учет URL, версии, обход зависимостей CSS, переписывание ссылок и проверка offline. [Playwright network](https://playwright.dev/docs/network), [request lifecycle](https://playwright.dev/docs/api/class-request).
- **chromedp:** разумная Go-native альтернатива, если запрещен Node runtime. Она также не является готовым зеркалировщиком. Node уже нужен для фронтенда, а Playwright можно использовать и в e2e-тестах. [chromedp](https://github.com/chromedp/chromedp).

### Граница обещания

Ни один подход не гарантирует полную автономную копию произвольного сайта: backend API, авторизация, WebSocket, DRM, CAPTCHA, геозависимость, ленивые ресурсы и пользовательские действия выходят за пределы одного снимка. Сохраненные скрипты могут зависеть от живого сервера. Скачивание JS и работа интерактивного приложения — разные требования.

Базовый режим: снимок отрендеренного состояния с оформлением, доступными ресурсами и отключенным исполнением стороннего JS при просмотре. Исходные HTML/JS можно хранить для архивной полноты. Режим со скриптами — отдельный изолированный replay, принимается только после проверки Browsertrix на реальном наборе сайтов. Если интерактивность обязательна уже в первой версии, P0 должен завершиться выбором replay-движка до реализации архивного блока.

### Сохранение путей и дедупликация

Идентичность сетевого ресурса учитывает `scheme + host + port + path + query`, метод и контекст запроса. `/style.css` на разных сайтах — разные ресурсы; один URL со временем может отдавать разные байты. Cookie и `Vary` тоже могут менять ответ.

Предлагается трехслойная схема:

1. **CAS (content-addressed storage):** неизменяемые blobs по SHA-256 содержимого.
2. **Manifest снимка:** исходный URL и исходный путь → конкретный blob, MIME, статус, параметры варианта.
3. **Дерево зеркала / gateway:** воспроизводит пути внутри namespace сайта и снимка, переписывая внешние и корневые ссылки. Для экспорта на одном Linux-volume можно создавать hardlink на CAS; исходные blobs никогда не менять на месте.

Таким образом, `/assets/site.css` сохраняется как логический путь, а одинаковые байты разных страниц используют один blob. Изменившийся файл получает новый hash и не портит старый снимок. Query string хранится отдельно в manifest; для каталога на диске — обратимое кодирование или suffix с hash query. Физический «один файл навсегда на каждый URL» несовместим с неизменяемостью снимков.

**Хранилище на старте:** локальный CAS на persistent volume. Это непосредственно соответствует Compose на одном сервере и допускает hardlinks. Через интерфейс `BlobStore` позже подключается S3 с AWS SDK for Go v2. Для самостоятельного S3 рассмотреть Garage с проверкой используемого подмножества API; не считать любой S3-compatible сервер полностью эквивалентным AWS. [Garage compatibility](https://garagehq.deuxfleurs.fr/documentation/reference-manual/s3-compatibility/).

MinIO не выбран по умолчанию: на проверенной странице GitHub основной репозиторий обозначен архивированным с 25 апреля 2026 года. Это конкретный сигнал сопровождения; перед выбором любого fork или коммерческого продукта нужна отдельная оценка. [Репозиторий MinIO](https://github.com/minio/minio).

## 8. Извлечение текста, Qdrant и RAG

**Extract:** `@mozilla/readability` + `jsdom` в capture-service, с отключенными скриптами и загрузкой внешних ресурсов в jsdom. Парсить сохраненный DOM, а не повторно скачивать страницу. Readability ориентирован на статьи; при плохом результате — очищенный main/body текст с отметкой качества. HTML для показа санитизируется, plain text для embeddings не требует HTML-render. [Readability и рекомендации безопасности](https://github.com/mozilla/readability).

**Index:** официальный `github.com/qdrant/go-client/qdrant`. В Qdrant сохраняются точки фрагментов с векторами и метаданными. Канонический извлеченный документ и версия остаются в PostgreSQL / blob storage, чтобы индекс можно было перестроить. [Go client](https://github.com/qdrant/go-client).

**Embedding:** стартовый локальный кандидат BGE-M3 через Ollama HTTP API. Он многоязычный; модельная карточка указывает dense-размерность 1024 и контекст до 8192 токенов. Это не доказательство лучшего качества на заметках пользователя. Начальный режим — только dense embeddings; наличие sparse/ColBERT в оригинальной модели не означает, что выбранный Ollama API выдает их. [Карточка BGE-M3](https://huggingface.co/BAAI/bge-m3), [Ollama model](https://ollama.com/library/bge-m3), [embed API](https://docs.ollama.com/api/embed).

Сравнить на размеченных RU/EN-запросах; фиксировать модель, digest, размерность, normalization и версию chunking. При смене модели — новая collection и переиндексация. Внешний embedding API возможен через тот же интерфейс после отдельного решения о передаче приватного текста.

Qdrant поддерживает `must`, `should`, `must_not` и payload-фильтры; индексировать поля фильтрации. Но копия ACL может отстать от PostgreSQL. Поэтому Qdrant возвращает ID/score, затем приложение проверяет актуальный доступ, статус удаления и версию, и только после этого загружает текст для пользователя или LLM. [Qdrant filtering](https://qdrant.tech/documentation/search/filtering/).

Генеративная модель, LangChain и LlamaIndex на этом этапе не нужны. Первое требование — надежный ingestion и retrieval; генерацию ответов можно добавить поверх проверенного контекста.

## 9. Frontend

**React + TypeScript strict + Vite SPA.** Собранные статические файлы отдает nginx; отдельный frontend Node-сервер в production не нужен. Node остается в capture-service. [Vite deployment](https://vite.dev/guide/static-deploy.html).

**TanStack Router:** типизированное состояние фильтров в URL, чтобы сохранять и пересылать поиск. **TanStack Query:** server state, mutations, invalidation и cursor pagination. **openapi-typescript + openapi-fetch + openapi-react-query:** Go DTO → OpenAPI → TypeScript types → клиент. Typescript-типы сами по себе не проверяют JSON во время исполнения. [Router search params](https://tanstack.com/router/latest/docs/guide/search-params), [OpenAPI client integration](https://openapi-ts.dev/openapi-react-query/).

**UI:** shadcn/ui и Tailwind CSS; код выбранных компонентов хранится в репозитории. **Формы:** React Hook Form + Zod; Zod используется для UX/URL, окончательная проверка остается в API. **Редактор:** Tiptap OSS с JSON как каноническим форматом rich-text и `schema_version`; платные collaborative-функции не требуются. Markdown можно реализовать отдельным kind/режимом позже. [shadcn/ui](https://ui.shadcn.com/docs), [Zod](https://zod.dev/), [Tiptap persistence](https://tiptap.dev/docs/editor/core-concepts/persistence).

Почему не Next.js: в заявленных требованиях нет SSR, а backend уже на Go; дополнительный серверный frontend-слой пока не оправдан. Если SEO публичных заметок, серверные OpenGraph previews и SSR становятся обязательны, пересмотреть frontend ADR. Vue + Vite тоже подходит; React выбран из-за совместимости выбранного набора редактора, компонентов и API-клиента, без утверждения о превосходстве по скорости.

## 10. Монорепозиторий, эксплуатация и инструменты разработки

Один Go module для API, worker и CLI; pnpm workspace для web, capture и generated API client; Go Task как общий запуск команд на Windows/Linux. Nx / Turborepo не добавлять до появления оправдывающего их объема сборки. [pnpm workspaces](https://pnpm.io/workspaces).

Docker Compose: nginx, api, worker, capture, PostgreSQL, Keycloak, Qdrant, Ollama, one-shot migrate; SMTP — внешний либо Mailpit в development. Сервисы разделены по процессам из-за доверия и ресурсов, а не из-за микросервисной бизнес-архитектуры. Readiness/healthcheck и `service_completed_successfully` задают порядок старта. [Compose](https://docs.docker.com/compose/how-tos/startup-order).

nginx: статический frontend, `/api/` и `/auth/` к Go; архивы на другом origin с авторизованной выдачей. Можно использовать API → `X-Accel-Redirect` → `internal` location или `auth_request`; выбранный образ должен содержать нужный модуль. [nginx proxy](https://nginx.org/en/docs/http/ngx_http_proxy_module.html), [auth_request](https://nginx.org/en/docs/http/ngx_http_auth_request_module.html).

Наблюдаемость: `log/slog` JSON сразу; OpenTelemetry traces и метрики очереди/capture/index. Prometheus/Grafana — отдельный ops profile при эксплуатации, не обязательны для локального запуска. Тесты: Go testing + Testcontainers PostgreSQL, Vitest/Testing Library, Playwright e2e. CI: gofmt, go vet, golangci-lint, govulncheck, tsc, ESLint, тесты, OpenAPI drift check. Версии этих дополнительных dev-tools фиксируются и проверяются при bootstrap; они не проходили отдельный сравнительный benchmark в этом ресерче.

Бэкап: PostgreSQL + blobs + конфигурация identity provider, регулярная проверка восстановления. Qdrant — snapshot для ускорения восстановления и возможность полной пересборки из канонического текста. Один Compose-host не дает high availability.

## 11. Версии и степень уверенности

Базовые линии: Go 1.27, PostgreSQL 18, Node.js 24 LTS; библиотеки выбирать в совместимых стабильных релизах. [Go release history](https://go.dev/doc/devel/release), [Node releases](https://nodejs.org/en/about/previous-releases). Не использовать плавающий `latest` в развертывании. Реальные patch-версии, image digests и dependency locks фиксируются после P0, а не придумываются в аналитическом документе.

Высокая уверенность: Bun/raw SQL, Huma/OpenAPI, реляционные теги, Compose, frontend type generation. Средняя: итоговая стоимость эксплуатации Keycloak и локальных embeddings без заданного бюджета. Требует прототипа: полнота зеркала на целевых сайтах, JS replay, качество извлечения, качество RU/EN retrieval, throughput и размер хранилища.
