# План реализации и приемки

Документы фиксируют рекомендуемое направление; P0 подтверждает наиболее рискованные стыки до создания полного приложения. Никакой этап ниже еще не считается выполненным только от наличия этого файла.

Обновление: foundation создан; локальные сборки и unit/browser checks выполнены. После перезапуска Docker прошли интеграционные проверки миграций, commit/rollback, River worker и HTTP smoke основного Compose. Подробности — [development.md](development.md). P0/P1 целиком не закрыты.

## P0 — проверочные прототипы

### P0.1 API / ORM / queue

Минимальный Huma endpoint с input/output DTO, ошибкой валидации и экспортом OpenAPI. PostgreSQL + Bun + pgx stdlib: создать note, выполнить raw SQL в той же транзакции, поставить River job через `tx.Tx`. Сделать rollback и убедиться, что ни note, ни job не появились. После commit Go worker через riverpgxv5 получает job. Сгенерированный TypeScript client компилируется и вызывает API.

Критерий: интеграционные тесты на реальном PostgreSQL, не только SQL mocks; валидный JSON ответ и ожидаемые ошибки; миграции с нуля проходят; базовые dependency versions зафиксированы. Если стык не работает, сначала исправить выбранный adapter, не менять весь стек без причины.

### P0.2 Архивирование — блокирующий технический риск

Создать локальные тестовые сайты: статическая статья, CSS imports/fonts/srcset, lazy images, SPA с API, redirects, два URL с одним asset, asset с тем же path и разными query, обновление CSS по старому URL. Добавить небольшой согласованный набор реальных целевых сайтов, не использовать его как единственную регрессию.

Сравнить Playwright DOM snapshot и Browsertrix/replay для динамического случая. Проверить offline reload с полностью отключенным внешним Интернетом, screenshot comparison и список отсутствующих ресурсов. Проверить, что raw HTML archive и reader text — разные артефакты.

Критерии:

- Две страницы с одинаковым asset используют один blob; изменение asset создает новую версию, старый snapshot не меняется.
- Все относительные/root-relative/CSS URL разрешаются в правильный snapshot/origin namespace.
- Query variants не перезаписывают друг друга; redirects и `<base>` покрыты fixtures.
- Неподдержанный интерактивный сценарий получает partial/unsupported, не ложный ready.
- SSRF fixtures для loopback, private IP, metadata, redirect на private IP, DNS rebinding, IPv6 и subresources заблокированы на уровне фактического egress.
- Archive-view не получает cookie приложения и не может запрашивать API с привилегиями пользователя.

Результат P0.2: отдельный ADR о поддерживаемых режимах и точная формулировка обещания сохранения. Не называть обычный DOM snapshot «полным интерактивным зеркалом». Если JS replay обязателен, его успешный прототип становится условием начала P3.

### P0.3 RAG ingestion

Readability на RU/EN статьях, техдокументации и странице без article. Ollama/BGE-M3 → Qdrant через Go SDK. Проверить фактическую размерность и предельный размер входа. Создать небольшую разметку запрос→релевантные документы; измерить recall@k, время и расход памяти на целевом железе. Сравнить хотя бы с PostgreSQL lexical baseline; альтернативную embedding-модель брать при неудовлетворительном качестве.

Критерий: повторный upsert не создает дубликаты; смена поколения не смешивает embedding spaces; private chunk никогда не попадает чужому viewer или LLM даже при искусственно устаревшем payload Qdrant.

## P1 — вертикальный срез заметок

Создать монорепу, Compose dev, migrator, Huma API, React SPA и API generation. Поднять Keycloak с email/password и одним OAuth provider, SMTP development. Создание/редактирование text note, теги, ALL/ANY/NONE, приватная библиотека, курсорная выдача.

Критерии: пользователь входит, создает заметку с произвольным числом тегов через batch operations, находит по трем включенным и двум исключенным тегам; API invalid input отвечает единообразно; frontend не содержит OIDC client secret; logout очищает кеш и сессию; исходные/сгенерированные API-типы не расходятся.

## P2 — социальная модель и ACL

Public/friends/unlisted/private; request/accept/remove friendship; лента; share token с отзывом/expiry; file note и защищенная выдача blob. UI явно показывает режим приватности до сохранения.

Обязательная матрица: owner / accepted friend / pending friend / stranger / anonymous × четыре visibility × list / GET / search / tags/count / file / archive / export / semantic retrieval. Каждый результат проверяется после изменения visibility, удаления и отзыва дружбы. Invalid/expired share не дает доступа; unlisted отсутствует в discover и public counts.

Cursor: повторные timestamp, одинаковые значения sort key, пустые страницы, смена фильтра, удаление элементов между страницами. Семантика тегов: пустые массивы, дубликаты, неизвестные теги, all∩none, any+none, одна метка у разных владельцев, unicode/case.

## P3 — web-архивы

Внедрить одобренный в P0 capture mode, immutable CAS, manifest, gateway, reader view, квоты, retries и UI статуса. Download/capture не блокирует note creation. Staging cleanup и mark/sweep garbage collection с grace period. Опциональный filesystem export сохраняет дерево и явно кодирует коллизии.

Критерии: все P0.2 fixtures в CI; повтор job и падение worker между записью blob и commit не портят данные; partial archive объясняет пропуски; ACL проверяется для CSS/image/fonts, а не только HTML. Старый grant теряет доступ после смены visibility; cache не обходит отзыв. Снимок остается воспроизводимым после повторного сохранения URL.

## P4 — Qdrant ingestion и семантический поиск

Extract/chunk/embed/index jobs, версии, reindex/delete, reconciliation task, метрики задержки и errors. Все web archives проходят ingestion; ошибки embeddings видны отдельно от archive state. API semantic search возвращает источники и разрешенные фрагменты. Генерация ответов RAG — следующий этап, не условие сохранения документов в Qdrant.

Критерии: восстановление индекса из исходных данных; отсутствие дублей при retries; текущий ACL перед выдачей текста; bounded overfetch без утечек через total/count; смена embedding model через новую collection. Преднамеренно задержанные sync-jobs не приводят к утечке закрытой заметки.

## P5 — эксплуатация

Production Compose, TLS/nginx, migration procedure, limits, backup/restore, health/readiness, audit events без секретов, OpenTelemetry. Уведомления о росте failed jobs, latency capture/index, заполнении диска и ошибках backup.

Нагрузочный профиль задается отдельно: ориентир тестового набора — 100 тыс. заметок, 1 млн связей тегов, неравномерное распределение и несколько размеров списка друзей. Это размер synthetic benchmark, а не прогноз пользователей. Сначала согласовать SLO API и бюджет ресурсоемких jobs; измерить p50/p95, планы SQL, queue lag, RSS Chromium/embedding и bytes per snapshot. Не объявлять стек высоконагруженным без результатов.

## Решения, которые можно принять позднее

- Конкретные OAuth providers и SMTP-сервис.
- Email/password либо magic link как основной UX.
- Нужно ли индексировать публичные страницы поисковиками; это влияет на SSR/SEO.
- Нужны ли собственные теги поверх чужих заметок; тогда добавить saved_items.
- Требуется ли полнофункциональный JS replay либо достаточно статического состояния.
- Список поддерживаемых сайтов, auth-required pages, размеры/retention архивов и вложений.
- Бюджет RAM/GPU; оставить BGE-M3, заменить модель либо использовать внешний embedding service.
- Один host/local CAS либо несколько hosts/S3. При переходе убрать предположение о shared local volume.

Для первоначальной реализации использовать дефолты из `stack.json` и архитектуры; не блокировать базовые notes/tags/API ожиданием этих ответов. Исключение — обещание полноты архивов: его необходимо подтвердить P0.2 до объявления поддержки.
