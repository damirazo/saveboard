# Индекс документов ссылок

`Index` вызывает локальный Ollama `/api/embed` и REST Qdrant; публичного поиска/RAG-chat здесь пока нет. Контракт модели в `backend/config/embedding-model.json` встраивается в Go-бинарник. Перед обработкой сверяются имя и digest установленной модели, количество/размерность/конечность ненулевых векторов. Модель BGE-M3: 1024, Cosine, коллекция `saveboard_bge_m3_790764642607_v1`. Изменение embedding space требует новой коллекции/переиндексации; оценка качества RU/EN пока не выполнена.

`post.rag` — отдельный River job. Он использует `/v1/extract` capture-сервиса: гостевой Chromium через прежний egress с проверкой каждого URL, затем Mozilla Readability в jsdom без выполнения скриптов и сетевых загрузок. Навигация, формы, скрытые блоки, скрипты, стили, footer/aside удаляются, сохраняется только нормализованный plain text. Минимум 80, максимум 100000 Unicode code points; DOM до 5 MiB. Это эвристическое выделение статьи, а не гарантия удаления всех рекламных/служебных фраз на каждом сайте. Пустая/нечитаемая страница завершается ошибкой, не пустым успешным индексом.

Канонический документ: `rag_documents` в PostgreSQL — source URL, title, text, chunks, generation UUID, модель, версия chunker, сроки и состояние. Повтор использует этот текст, чтобы одна генерация не смешивала разные версии страницы. Chunks до 1600 code points, overlap 200; пакет до 8. Qdrant содержит детерминированные UUID точек от generation + ordinal, вектор и payload `post_id`, `owner_id`, `generation_id`, `chunk_index`, `text`, `title`, `source_url`, `model_digest`, `chunker_version`. Upsert с `wait=true`, совместимость коллекции проверяется до записи.

## Граница доступа и жизненный цикл

`post_rag.revision` — указатель на актуальную генерацию. До `rag_documents.state=ready` частичные точки не являются готовым документом. Будущая выдача обязана перепроверить в PostgreSQL active revision, published/link, enabled и текущий ACL для каждого кандидата **до выдачи текста/count/snippet**. Qdrant payload не является ACL; сервис закрыт внутренней сетью и ключом. Архивный текст — недоверенные данные, не инструкции для будущего RAG-агента.

Включение/смена URL/повтор после failed создаёт документ и задачу в SQL-транзакции поста. Название/видимость не пересоздают текст, поскольку права всегда проверяются по текущему посту. Выключение, смена типа и draft сразу убирают active revision. Старые точки чистятся в фоне по generation filter. Hard delete также поддерживается: документы не каскадно исчезают раньше удаления внешнего индекса.

Advisory lock поколения исключает одновременные upsert и очистку без блокировки CRUD. После удаления Qdrant очищается текст документа; компактная запись поколения остаётся как tombstone и проверяется повторно раз в час, чтобы исправить неоднозначный сетевой сбой/запоздалую внешнюю запись. Недоступность Qdrant оставляет очистку для следующего прохода. Публичного доступа к таблице документов нет.

Три попытки (паузы 15/60 секунд), попытка 5 минут, общий срок 30 минут с учётом очереди. Capture/RAG делят один слот `capture` в текущем worker. Общий River rescue увеличен до 6 минут, чтобы не восстанавливать живую RAG-попытку раньше её таймаута; таймаут собственно capture остался 2 минуты. Сверка раз в 30 секунд обрабатывает срок, отсутствующее/исчерпанное задание, отмену и очистку. Успех/конечная ошибка и персональное уведомление фиксируются атомарно; ключ `rag:<generation>` предотвращает дубли.

## Запуск и проверки

Worker: `CAPTURE_URL`, `CAPTURE_TOKEN`, `OLLAMA_URL`, `QDRANT_HOST`, `QDRANT_API_KEY` и PostgreSQL. Модель устанавливается штатным `docker compose --env-file .env -f deploy/compose.yaml --profile embeddings run --rm model-init` из корня (скачивание около 1.2 GB только при отсутствии). Миграция `00008_link_rag.sql` обязательна. Недоступные сервисы не мешают CRUD.

`go test ./internal/search` и `go test -tags=integration ./internal/posts -run TestRAGLifecycle` из backend; capture unit/browser smoke; `frontend/e2e/posts.spec.ts` проверяет форму. API-контракт генерируется корневой `pnpm api:generate`.

Источники: [Mozilla Readability](https://github.com/mozilla/readability), [Qdrant points](https://qdrant.tech/documentation/concepts/points/), [Ollama embeddings](https://github.com/ollama/ollama/blob/main/docs/capabilities/embeddings.mdx).
