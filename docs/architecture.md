# Архитектурная спецификация Saveboard

Статус: предложение для реализации; 2026-10-07. Обоснование библиотек и первичные источники — [research.md](research.md). Здесь описано проектное поведение, которое еще предстоит реализовать и протестировать.

Базовый каркас добавлен. Фактический объём и проверки — [development.md](development.md); эта спецификация по-прежнему описывает целевое поведение.

## 1. Границы системы

Модульный монолит Go с модулями identity, notes, tags, access, feed, archives, search и jobs. HTTP handlers вызывают application services; services управляют транзакциями и политикой доступа; repositories исполняют Bun/SQL. Не создавать универсальный generic repository, скрывающий полезные возможности SQL.

Поток записи ссылки:

```text
React → nginx → Go API → PostgreSQL (note + snapshot request + River job)
                                      ↓
                                 Go worker
                                      ↓ private authenticated HTTP
                              Node capture-service → isolated Chromium → Internet
                                      ↓ streamed capture bundle
                                 Go worker → immutable CAS + manifest in PostgreSQL
                                      ↓
                               extracted document → Ollama → Qdrant
```

Capture-service не получает доступ к PostgreSQL, Qdrant, пользовательским сессиям или Docker socket. Go worker оркестрирует River и принимает результаты через внутренний HTTP-контракт. Повторный capture использует idempotency key; промежуточные результаты живут во временном хранилище до атомарной публикации manifest.

## 2. Модель данных

### Пользователи и связи

- `users(id, display_name, created_at, disabled_at)`.
- `identities(user_id, issuer, subject)`, UNIQUE `(issuer, subject)`. Email может храниться как профильный verified атрибут, но не как внешняя primary identity.
- `sessions(token_hash, user_id, expires_at, revoked_at, created_at)`; секреты OIDC при необходимости — зашифрованное серверное хранилище.
- `friendships(user_low, user_high, status, requested_by, created_at)`, CHECK `user_low < user_high`, UNIQUE пары; status pending / accepted. Отдельный факт блокировки — `user_blocks(blocker_id, blocked_id)`, если продукт вводит блокировки. Ниже SQL не предполагает существование блокировок.

### Заметки и теги

- `notes(id uuid, owner_id uuid, kind text, visibility text, title text, created_at timestamptz, updated_at timestamptz, deleted_at timestamptz, version bigint)`.
- `kind`: первоначально `text`, `web`, `file`. «Статья» может быть web-заметкой либо классификацией; первоначальный фильтр «статьи» трактуем как `kind=web`, а при необходимости добавим `content_class=article` независимо от kind.
- `visibility`: `private`, `friends`, `unlisted`, `public`; default private. CHECK-constraints в миграциях, Go enum DTO. Text + CHECK проще расширять, чем привязывать всю систему к PostgreSQL ENUM.
- `text_notes(note_id PK/FK, content_json jsonb, schema_version int, plain_text text)`; Tiptap document JSON с проверкой структуры.
- `web_notes(note_id PK/FK, original_url text, normalized_url text, current_snapshot_id uuid NULL)`.
- `file_notes(note_id PK/FK, blob_id, original_name, detected_mime, size_bytes)`.
- `tags(id uuid, owner_id uuid, name text, normalized_name text)`, UNIQUE `(owner_id, normalized_name)`.
- `note_tags(note_id uuid, tag_id uuid)`, PK `(note_id, tag_id)`, обратный index `(tag_id, note_id)`.

Все kinds используют одну `notes` и `note_tags`: новый тип заметки автоматически получает общие теги и ACL. JSONB используется для версионируемого rich-text/редких метаданных, а не вместо связей и индексируемых ACL. Инвариант «одна subtype-строка соответствует kind» реализуется транзакционным сервисом и проверяется тестами; при записи через другие сервисы добавить deferred constraint trigger.

**Теги первоначально принадлежат владельцу заметки.** Их назначает владелец. Для публичной ленты поиск по нормализованному имени находит соответствующие tag IDs разных авторов; эти ID нельзя случайно отфильтровать только по текущему viewer. Для своей библиотеки можно искать по своим tag IDs. Автодополнение общих тегов использует только видимые заметки, чтобы не раскрывать приватные названия.

Нормализация: trim, Unicode NFC, case folding; сохранить исходное отображение, уникальность применять к normalized value. Уточнить правила длины и запрещенных управляющих символов. Если пользователю нужно теггировать чужую запись своими тегами, ввести отдельную сущность `saved_items(user_id, note_id)` и ее tags; не менять чужой `note_tags`.

В модели нет фиксированного числа тегов. Для защиты API вводится документированный лимит на одну операцию (например 100 элементов в фильтре), размер body и batch size; это не ограничивает общее число тегов заметки. Значения лимитов — стартовые настройки, не результат нагрузочного теста.

### Архивы и индекс

- `archive_snapshots(id, note_id, requested_url, final_url, captured_at, state, manifest_blob_id, capture_version, failure_code, warnings, idempotency_key)`.
- `archive_resources(snapshot_id, resource_key, original_url, original_path, query_string, origin_key, blob_id, replay_blob_id, mime, status_code, variant_key)`, UNIQUE `(snapshot_id, resource_key)`.
- `blobs(id, sha256, storage_key, size_bytes, created_at)`, UNIQUE sha256 в пределах выбранного dedup scope.
- `extracted_documents(id, note_id, snapshot_id, content_version, extractor_version, language, text_blob_id, text_hash, state)`.
- `document_chunks(id, document_id, ordinal, content_hash, text_blob_id, token_count, section_path, start_offset, end_offset)`.
- `index_generations(id, document_id, model_id, model_digest, chunker_version, collection, state)`.
- `note_shares(id, note_id, token_hash, expires_at, revoked_at)`; доступ по ссылке не основан на угадываемом note ID.

`snapshot.state` и `index_generation.state` независимы. Успешно сохраненная страница остается доступной, если embeddings временно недоступны. Удаление заметки сразу исключает ее из доступа, затем асинхронно удаляет производные данные.

## 3. Политика доступа

У каждого запроса есть viewer (анонимный либо user ID). По умолчанию отказ. Владелец может читать свои заметки всех режимов и изменять их. Остальные:

- `public`: чтение, поиск и общая лента доступны всем.
- `friends`: чтение и поиск только при accepted-связи viewer ↔ owner; попадает в ленту этого друга.
- `unlisted`: только через действующий share token; не участвует в общей ленте, глобальном поиске, sitemap, публичных tag counts или глобальном RAG. Владелец находит в своей библиотеке. Простой URL с UUID недостаточен.
- `private`: только владелец.

Доступ по share token привязан к note и его текущему visibility=unlisted; перевод в private отзывает прежний режим. Токены генерируются криптографически (минимум 256 случайных бит), в БД хранится hash, возможны ротация и отзыв. Token не записывается в обычные access logs/analytics; `Referrer-Policy: no-referrer`, запрет индексации — дополнительные меры, не замена ACL.

Разделять `canReadByID`, `canDiscover`, `canEdit`, `canReadArchive`. Владелец может видеть private/unlisted в своей библиотеке, но общая лента содержит только public и допустимые friends. Общий feed predicate поэтому уже, чем predicate чтения.

Проверять ACL для списка, прямого GET, поиска, counts, подсказок, экспорта, вложений, thumbnails, каждого ресурса архива и каждой передачи текста в RAG. Raw SQL не проходит автоматически через ORM-scopes: условие доступа обязательно и тестируется для каждого repository entrypoint. В первой версии RLS не обязателен; его добавление требует отдельной схемы `SET LOCAL` в транзакции и роли без BYPASSRLS, а не надежды на owner-role таблиц.

При отзыве дружбы или смене visibility PostgreSQL применяется сразу к последующим запросам. Уже скачанные пользователем данные технически отозвать нельзя. Не кешировать приватные ответы публично; при требовании быстрого отзыва серверный cache должен инвалидироваться или быть выключен. Кеш frontend очищается при logout / смене аккаунта; query keys включают identity/scope.

## 4. Поиск и лента

Контракт `POST /api/v1/notes/search`:

```json
{
  "scope": "library",
  "kinds": ["web"],
  "tags": {
    "all": ["golang", "postgresql", "architecture"],
    "any": [],
    "none": ["draft", "outdated"]
  },
  "text": "",
  "limit": 30,
  "cursor": null
}
```

Это проектный пример, не уже реализованный endpoint. Tags здесь — нормализуемые имена; внутренний SQL работает с разрешенными множествами UUID.

`all=[A,B,C]` означает присутствие каждого. `none=[X,Y]` означает отсутствие любого из X/Y: `NOT X AND NOT Y`. Это отличается от `NOT (X AND Y)`. Пустой `all` не ограничивает выборку; пустой `any` не ограничивает; пустой `none` ничего не исключает. Дубликаты удалить; пересечение all и none — пустая выдача. Неизвестный all-tag дает пустую выдачу; неизвестный none-tag ничего не исключает. Для одинакового имени у разных владельцев объединять IDs внутри условия одного имени, а не требовать все эти IDs одновременно.

`scope=library` — заметки владельца. `scope=discover` — public + friends, без private/unlisted, включая заметки самого viewer только если режим подходит. Для anonymous только public. `GET /api/v1/feed` использует discover и keyset pagination.

Параметризованный [пример SQL](../backend/docs/examples/search-notes.sql) реализует фильтрацию по IDs, `EXISTS` и `NOT EXISTS`, актуальную дружбу и keyset. Для поиска имен в discover предикат строится по группам нормализованных имен через JOIN tags; нельзя просто развернуть одно имя в all_ids. SQL в примере — проверяемая отправная точка, не обещание лучшего плана для всех распределений данных.

Индексы-кандидаты:

```sql
CREATE INDEX notes_owner_created ON notes (owner_id, created_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX notes_public_created ON notes (created_at DESC, id DESC)
  WHERE deleted_at IS NULL AND visibility = 'public';
CREATE INDEX notes_friends_created ON notes (owner_id, created_at DESC, id DESC)
  WHERE deleted_at IS NULL AND visibility = 'friends';
CREATE INDEX note_tags_tag_note ON note_tags (tag_id, note_id);
CREATE INDEX friendships_by_high ON friendships (user_high, user_low)
  WHERE status = 'accepted';
```

PK friendships покрывает low→high. Cursor содержит `(created_at, id)` и привязку к фильтру/scope, без OFFSET для длинной ленты; fetch `limit+1`. UUID сравнивается только для tie-break, не как время. Для text-search выбрать конфигурацию языка документа и GIN; RU/EN/код требуют отдельной проверки качества токенизации. При сложной ACL OR-выборке проверить альтернативный UNION ветвей со стабильной дедупликацией.

## 5. API-контракты и валидация

Базовые endpoints: `/auth/login`, `/auth/callback`, `/auth/logout`, `/api/v1/me`; CRUD `/notes`, `/tags`; поиск `/notes/search`; `/feed`; `/friendships`; `/notes/{id}/shares`; `/notes/{id}/archives` и `/archives/{id}`; позднее `/search/semantic`. Все state-changing endpoints требуют CSRF-защиты при cookie auth.

Иллюстрация Huma DTO (не самостоятельная компилируемая программа):

```go
type CreateWebNoteInput struct {
    Body struct {
        Title      string   `json:"title" minLength:"1" maxLength:"500"`
        URL        string   `json:"url" format:"uri" maxLength:"8192"`
        Visibility string   `json:"visibility" enum:"private,friends,unlisted,public" default:"private"`
        TagIDs     []string `json:"tag_ids,omitempty"`
    }
}

type CreateWebNoteOutput struct {
    Body struct {
        ID           string `json:"id" format:"uuid"`
        ArchiveState string `json:"archive_state" enum:"pending,running,ready,partial,failed"`
    }
}
```

Для создания web note — HTTP 201 с note и pending archive; длительная работа идет асинхронно. Отдельный запрос повторного архивирования — HTTP 202. Нельзя держать API-соединение открытым до завершения Chromium.

Huma проверяет форму входа; сервис отдельно проверяет UUID tags, принадлежность, duplicates, разрешенную схему URL http/https и ограничения тела. `format:uri` не является SSRF-защитой. Для PATCH учитывать различие absent/null/zero; `version` либо ETag/If-Match предотвращает потерянные обновления. Ошибки единообразные Problem Details с code и field errors; не отдавать SQL, filesystem paths или cookies в ошибках.

Экспорт OpenAPI выполняется CLI без поднятия БД. Генерация frontend types в CI; повторная генерация не должна оставлять diff. Схема API — источник wire-типов, Zod-формы не становятся второй независимой схемой всего backend.

## 6. Архивирование и пути

### Capture protocol

Go worker передает внутреннему capture-service job ID, URL, capture policy и квоты. Контракт версионируется. Никаких произвольных shell-команд или пользовательских аргументов Chromium. Service возвращает DOM, перечень фактически полученных ресурсов, исходные URL, статусы, MIME, extraction result и warnings; тела — потоком/временным bundle с проверкой лимитов и hash. Staging-файлы удаляются по retention policy.

Capture работает в свежем browser context. Начальный scope — одна страница с зависимостями, а не бесконечный recursive crawl всего сайта. Политика lazy scroll ограничена числом шагов/временем. Снимок после bounded readiness; `networkidle` не считать достаточным или всегда достижимым. Рекурсивно обработать CSS `@import` и `url()`, `srcset`, fonts, SVG/image references, `<base>`, redirects, nested frames; отсутствующие ресурсы явно перечислить.

Использовать HTML/CSS parsers, не regex. Конкретный adapter: jsdom для DOM, PostCSS + postcss-value-parser для CSS URL. Версии/edge cases этих parser-пакетов проверить в P0. Service workers по умолчанию блокировать для наблюдаемого capture; сайты, зависящие от них, помечать несовместимыми с этим режимом, а не выдавать ложный success.

### Идентичность и версии

Сохранить original URL и final URL после redirect. Нормализация URL консервативная: host lower-case, default port handling; не удалять query, не сортировать ее автоматически, не склеивать `/a`, `/a/` и `/A`. Fragment не уходит в HTTP, но влияет на навигацию в SPA: сохранять его в page metadata.

Один request key: URL без fragment + method + digest тела при наличии + hash контекста представления. Запросы с cookies/authorization нельзя переиспользовать только по URL. В первой версии пользовательские браузерные cookies не импортируются. HTML может быть персонализированным даже без явной авторизации.

Одинаковый URL допускает повторную загрузку: дедупликация места на диске не означает всегда допустимое кеширование сети. ETag / Last-Modified / cache policy используют для revalidation с учетом Vary; ложная экономия запросов не должна сохранять устаревшую страницу. Дедупликация производится после получения байтов и не раскрывается клиенту через lookup-by-hash API.

CAS хранит исходные тела как immutable blobs. Политика декодирования content-encoding единая и версионированная; MIME/encoding/headers — в manifest. Для переписанных HTML/CSS создаются отдельные derived blobs. Не отдавать stale Content-Encoding/Content-Length/Set-Cookie/CSP исходного ответа поверх измененных байтов; HTTP-заголовки формирует gateway. SRI-хэши переписанных ресурсов пересчитать или удалить в производном документе согласно capture policy.

### Логические пути

Пример оригиналов:

```text
https://example.org/articles/a.html
https://example.org/assets/site.css?v=1
https://cdn.example.net/fonts/f.woff2
```

Gateway адресует их так:

```text
https://archive.example.net/a/{snapshot}/{origin-key}/articles/a.html
https://archive.example.net/a/{snapshot}/{origin-key}/assets/site.css?v=1
https://archive.example.net/a/{snapshot}/{cdn-origin-key}/fonts/f.woff2
```

Это иллюстративные домены, не выбранные DNS-имена. Namespace отделяет snapshots/origins; suffix пути внутри origin сохранен. Все ссылки переписываются с учетом document base и CSS base, включая `/root-relative`, protocol-relative и absolute URLs. Обычные относительные ссылки сохраняются, где разрешаются корректно. Два снимка могут ссылаться на один CAS blob; новый CSS использует другой blob.

Сохранение буквально всех имен на физической файловой системе невозможно без кодирования коллизий: query, `/a` как файл и `/a/b` как вложенный путь, escaped slash и Windows-reserved names. Источник истины — manifest с точным URL; filesystem export детерминированно кодирует такие случаи. Path traversal и повторное URL decoding запрещены. Nginx не должен напрямую собирать путь к blob из пользовательского URL.

### JS и изоляция просмотра

Базовый snapshot-view — сохраненный DOM, стили и медиа, без исполнения JS; оригинальные scripts могут оставаться в CAS. JS replay не реализовывать простым добавлением `allow-scripts`: произвольный JS может строить URL динамически, обращаться к backend, регистрировать service workers и менять DOM. P0 сравнивает Browsertrix/replay на целевом корпусе; отдельный ADR определяет его формат и интеграцию с CAS. Не обещать межархивную дедупликацию внутри WACZ без проверки.

Архивы обслуживать на отдельном origin, предпочтительно отдельном registrable domain. Cookie приложения host-only (`__Host-` при HTTPS). Просмотр в sandbox iframe; в базовом режиме запрет scripts, forms, popups и top navigation. CSP ограничивает загрузки архивным хостом; никаких живых внешних fallback-запросов. Безопасность CSP не заменяет серверную авторизацию каждого ресурса.

API после проверки ACL выдает одноразовый bootstrap grant; архивный gateway обменивает его на собственную короткую серверную сессию, привязанную к note/snapshot/viewer. Для unlisted grant дополнительно привязан к действующему share. Каждый запрос ресурса повторно проверяет текущий ACL/share; короткий TTL сам по себе не обеспечивает немедленный отзыв. Cookie архива не открывает API приложения. Учитывать блокировку third-party cookies: fallback — top-level просмотр на archive origin с тем же bootstrap обменом, без ослабления ACL.

### Безопасность capture

URL задается пользователем, поэтому браузер — источник SSRF-риска. Только http/https; блокировать loopback, private/link-local/metadata endpoints, IPv4-mapped IPv6 и служебные Docker-сети. Проверять redirects и все subrequests, повторно проверять DNS/адрес фактического соединения. Прикладной проверки URL недостаточно: egress proxy/firewall должен разрешать браузеру только публичный Интернет, без обхода через прямые TCP/UDP/WebRTC/QUIC соединения. Capture API слушает отдельно от egress browser namespace. [Основание: OWASP SSRF](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html).

Docker-сеть с красивым именем сама этого не обеспечивает. До публикации проверить сетевой запрет обращения к host, RFC1918, cloud metadata и другим контейнерам. Chromium запускается non-root с рабочим sandbox, без `--no-sandbox`; browser subprocess не получает сервисные credentials. Ограничить CPU/RAM/pids, wall-clock, redirects, число/размер ресурсов и размер распакованных данных. Ошибки квот → partial/failed с объяснением.

Физический dedup — доверенный backend-механизм. Даже если blob общий, читать его можно только через разрешенный manifest. Публичного endpoint `/blobs/{hash}` нет. Garbage collection удаляет только неиспользуемые blobs после grace period; конкурентный capture защищен staging lease/ссылкой.

## 7. Pipeline Qdrant

1. Capture публикует immutable snapshot.
2. Extraction создает канонический документ, language, title, quality и версии parser/capture.
3. Chunker делит по заголовкам/абзацам, не теряя код/таблицы; стартовая гипотеза 400–800 model tokens и overlap 50–100. Проверять tokenizer выбранной модели; не считать символы токенами и не допускать silent truncation.
4. Embedder получает chunks локально, сохраняет model digest / vector dimension / normalization policy.
5. Indexer upsert-ит точки с детерминированным UUID от document version + chunk ordinal + embedding generation; чужие поколения не перезаписываются.
6. После подтверждения записи Qdrant активирует generation в PostgreSQL. При поиске старые/неактивные generations отбрасываются. Затем удаляются старые точки. Повтор каждой стадии безопасен.

Payload: `note_id`, `owner_id`, `snapshot_id`, `document_id`, `content_version`, `generation_id`, `chunk_index`, `kind`, `visibility`, `tag_ids`, `language`, `captured_at`. Текст chunks хранить канонически вне Qdrant; при желании дублировать в payload, но не возвращать до ACL-check. Payload indexes для note/owner/kind/visibility/tags/generation согласно запросам. Collection на embedding space, не отдельная collection на каждую заметку/пользователя. Первоначально cosine/dense 1024 для BGE-M3, с проверкой фактического выхода выбранного runtime.

Qdrant не выполняет embedding только от факта записи текста: в выбранной self-hosted конфигурации worker явно вызывает Ollama. В индекс попадают документы всех режимов, включая private/unlisted; discover/RAG исключает unlisted, кроме явно заданного контекста разрешенного shared-документа. Qdrant закрыт внутренней сетью и credentials.

Semantic retrieval: из PostgreSQL получить актуальный контекст viewer и friends; сформировать предварительный Qdrant-filter по owner/visibility/tag constraints. Запросить ID + score без text; проверить их в PostgreSQL, применить текущие теги, ACL, deletion, document generation. Только разрешенные IDs превращаются в текст и citations. Добрать кандидатов при отбрасывании; установить bounded budget и честно вернуть меньше результатов, если бюджет исчерпан. Не показывать необработанное Qdrant total/count.

Изменения visibility/tags/deletion атомарно ставят sync-job в PostgreSQL. Дружба не требует переписывать массив friend IDs в каждом point: список доступных owners рассчитывается при запросе. Отстающий индекс может временно пропустить ставший доступным документ, но не должен выдать ставший недоступным. Финальная проверка PostgreSQL обязательна даже при prefilter.

Будущий RAG получает только уже разрешенный контекст, с URL/даты/версии для цитирования. Текст архивов считается недоверенными данными, а не инструкциями агенту. Логи не содержат приватные chunks, URL share tokens или embedding payload.

## 8. Auth flow

Go `/auth/login` создаёт state + nonce + PKCE S256 verifier и направляет в Keycloak. `/auth/callback` проверяет browser-binding cookie, state, issuer, audience, expiry, подпись и nonce; меняет code сервером; находит identity по issuer/subject; создаёт сессию и возвращает JSON с access JWT и одноразовым refresh-токеном. Cookie применяется только для OAuth flow (Secure при HTTPS, HttpOnly, SameSite=Lax). Вход по локальному username/email: `POST /api/v1/auth/login`; ротация: `POST /api/v1/auth/refresh`. `POST /api/v1/auth/logout` отзывает текущую сессию, `POST /api/v1/auth/logout-others` — остальные; оба требуют Bearer JWT. `GET /api/v1/auth/me` проверяет текущий JWT и сессию. Проверка статуса сессии в PostgreSQL обеспечивает немедленный отзыв. Cookie-авторизация API отсутствует; если она будет добавлена, нужны CSRF token/Origin validation. Локальный logout не завершает SSO-сессию Keycloak. Детали: [identity README](../backend/internal/identity/README.md).

SMTP обязателен для email verification/recovery. OAuth credentials и redirects конфигурируются для конкретных провайдеров; секреты не в frontend bundle. Keycloak — отдельная logical DB/user внутри PostgreSQL на одном host; shared PostgreSQL не означает shared schema. Public issuer должен совпадать с проверяемым issuer, даже если containers используют внутренний адрес.

## 9. Монорепозиторий и Compose

Текущие границы каталогов (перечень feature-модулей в спецификации описывает также будущую реализацию):

```text
backend/go.mod / go.sum         # один прикладной Go module
backend/cmd/ backend/internal/ backend/migrations/
backend/capture/                # Node, Playwright, Readability
backend/scripts/ backend/config/
backend/api/openapi.json        # экспорт контракта
backend/pnpm-workspace.yaml / pnpm-lock.yaml
frontend/src/ frontend/e2e/     # React SPA и browser tests
frontend/packages/api-client/  # generated types + HTTP client
frontend/api/openapi.json      # входной снимок контракта
frontend/pnpm-workspace.yaml / pnpm-lock.yaml
backend/Dockerfile backend/capture/Dockerfile frontend/Dockerfile
frontend/nginx.conf
deploy/compose.yaml deploy/keycloak/ deploy/postgres/
Taskfile.yml scripts/ tools/ docs/  # общая координация и документация
```

Backend и frontend имеют независимые dependency graphs и Docker build-контексты. Фронтенд не читает Go-исходники: интеграция проходит через экспортированный OpenAPI snapshot. Общий сценарий `scripts/sync-api.mjs` синхронизирует артефакт в монорепозитории; после выделения репозиториев его заменяет доставка контракта через CI. Общего pnpm workspace между частями нет.

Compose starts PostgreSQL healthcheck → migrate one-shot → api/worker. Keycloak и Qdrant имеют readiness checks. Ollama отдельно получает закрепленную model version и проходит embed smoke test, не только проверку открытого порта. API CRUD остается работоспособным при недоступности embedding/capture: очередь ожидает восстановления, UI показывает статус.

nginx собирается с frontend dist в image; `/api/` не попадает в SPA fallback. `/auth/` → Go, identity domain → Keycloak. TLS и renew сертификатов обеспечиваются на edge; в локальной разработке отдельная конфигурация. Наружу только необходимые nginx 80/443; DB/Qdrant/capture/Ollama/admin UI не публикуются. Archive gateway может быть handler того же Go-бинарника с отдельной route group, отдельным hostname и отдельной сессией.

Постоянные volumes: PostgreSQL, CAS, Qdrant, модели Ollama. Browser temp — ephemeral. Только worker пишет CAS; nginx читает только по internal redirect, после проверки manifest и доступа Go. При S3 backend Go проксирует поток или использует проверенную короткую выдачу; долгие presigned URLs несовместимы с требованием мгновенного отзыва.

В CI фиксировать Go module versions, pnpm lock, Docker digests, Chromium/Playwright matching version, model digest. Секреты из env/secrets files вне Git; `.env.example` содержит только имена и безопасные примеры. Backup/restore проверяется, including Keycloak DB и CAS, а не только `docker compose up`.
