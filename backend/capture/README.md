# Capture

Статический DOM после гостевого Chromium-рендера и независимый PNG-эскиз 800×600. API ставит River-задачу, worker вызывает `POST /v1/captures` с `url`, `save_copy`, `create_thumbnail`. Bearer CAPTURE_TOKEN обязателен. Capture возвращает HTML, resources manifest и PNG; долговременный CAS записывает только worker.

`POST /v1/extract` с `{url}` использует тот же изолированный рендер и общий слот запросов, затем Readability возвращает `{title,text,url}`. Это отдельная операция для RAG без сохранения HTML/эскиза. jsdom работает без scripts/resources; результат plain text. Подробности и лимиты — [search README](../internal/search/README.md). Browser smoke проверяет текст, созданный JavaScript, и исключение navigation/footer.

Запускать через `deploy/compose.yaml`: capture — только в internal-сети; capture-egress — авторизованный шлюз с выходом в сеть, без БД. `CAPTURE_EGRESS_URL` обязателен для включения; без него endpoint остаётся 501. Не подключать capture напрямую к внешней Docker-сети. Оба сервиса используют один CAPTURE_TOKEN. `CAPTURE_CACHE_ROOT` шлюза хранит ограниченный persistent CSS-кэш; runtime образ пишет только в /tmp, шлюз также в /cache.

Правила SSRF, offline-просмотра, immutable/dedup и ограничения: [ADR-0002](../../docs/decisions/0002-static-link-captures.md).

Проверки из корня:

```sh
pnpm --dir backend --filter @saveboard/capture test
docker compose --env-file .env -f deploy/compose.yaml build capture
docker compose --env-file .env -f deploy/compose.yaml up -d --no-build --wait capture capture-egress
docker compose --env-file .env -f deploy/compose.yaml exec -T capture node capture/dist/browser-smoke.js
```

Browser smoke использует контролируемые ответы fixture-шлюза для DOM/CSS/JS, а private/metadata/subresource запросы отправляет настоящему egress-шлюзу; отдельно проверяет отсутствие прямого DNS/IPv4 egress контейнера. Chromium и Playwright закреплены одной версией Dockerfile/package.json. Нельзя считать одну сборку доказательством сетевой изоляции.
