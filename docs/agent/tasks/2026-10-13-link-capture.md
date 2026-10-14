# Фоновые копии ссылок и эскизы

Статус: done. Запрос и завершение: 2026-10-13.

## Результат и приёмка

- Два независимых флажка, по умолчанию выключены; сохранение поста не ждёт браузер.
- River job в транзакции поста; до успешного завершения оба результата отсутствуют. Устаревшая задача не публикует результат после изменения URL/опций/типа.
- Chromium выполняет скрипты при захвате, сохраняется статический DOM, CSS и ресурсы. При просмотре отсутствуют скрипты и внешние запросы; изолированный sandbox без доступа к origin приложения.
- Общие CSS переиспользуются; immutable CAS и manifest с хэшами домена/пути и отдельными версиями для query/content.
- Эскиз 800×600, кнопка копии и полная ссылка (отображение до 300 символов). ACL каждого чтения соответствует посту.
- SSRF: pin IP после DNS, запрет непубличных адресов/нестандартных портов, все редиректы/подресурсы через шлюз, браузер без прямого egress.

## Этапы

- [x] Изучить текущие сервисы и уточнение о фоновой задаче.
- [x] Миграция, транзакционная очередь, worker и ACL выдачи.
- [x] Capture и egress шлюз, статическая сериализация, dedup.
- [x] Форма, отображение, изолированный просмотр.
- [x] Unit/integration/browser и проверка сетевой изоляции в Compose.
- [x] Обновить документацию и контекст.

## Исходное состояние

Capture был отключён (501), River выполнял только receipts. В рабочем дереве есть незакоммиченная реализация auth-форм; сохраняем её. БД на миграции 5. Новая задача не предполагает push.

## Ограничения

Произвольные сайты могут требовать вход, CAPTCHA, бесконечную прокрутку, интерактивные iframe или долгие запросы. Захват ограничен временем/размером; это статический снимок гостевого просмотра. IPv6 не поддерживается. Полные ограничения и хранение — [ADR-0002](../../decisions/0002-static-link-captures.md). GC непривязанных blobs и квоты долгосрочного хранения остаются отдельной задачей. Remote GitHub CI не запускался; локальные проверки ниже.

## Проверки 2026-10-13

Из корня:

- `pnpm task check` → passed: типы, lint/vet, unit, сборки, контракт. После последних изменений capture/alias повторён `pnpm --dir backend check` → passed. После уточнения optional flags — frontend typecheck и `node scripts/check-contract.mjs` → passed.
- `pnpm task test:integration` → passed. После финальных правок отдельно `go -C backend test -tags=integration -count=1 ./internal/posts` и `./internal/transport/httpapi` → passed. Тесты покрывают транзакцию/rollback River, смену URL во время работающего capture, idempotence, независимые flags, failed/retry, private/draft ACL.
- `pnpm test:e2e` с `PLAYWRIGHT_CHANNEL=chrome` → passed, 10 сценариев. API в этих тестах подменён; проверены desktop/mobile, polling, CSS/image в opaque iframe, отсутствие выполнения scripts и сетевых запросов при просмотре.
- `docker compose --env-file .env -f deploy/compose.yaml build api capture nginx` → passed; `up -d --no-build --wait --wait-timeout 180 api worker capture capture-egress nginx` → passed, healthy. БД обновлена до миграции 6, volumes сохранены.
- `docker compose --env-file .env -f deploy/compose.yaml exec -T capture node capture/dist/browser-smoke.js` → passed на финальном образе: реальный Chromium, DOM после JS, open shadow root, PNG 800×600, независимые flags, общий CSS, private/metadata/IPv6, SSRF через подресурс и редирект, запрет прямого DNS/IPv4 egress.
- `node scripts/smoke-compose.mjs` → passed на финальном запуске.
- `node tmp/check-live-capture.cjs` (локальный игнорируемый smoke) → passed: настоящая регистрация через браузер, POST link → pending, River → capture → CAS, эскиз и приватный offline viewer. Гость получает 404. Временные аккаунт, пост, тег и job удалены.
- `docker compose ... config --quiet`, `git diff --check` → passed.

Во время разработки исправлены обнаруженные проверки: opaque sandbox не читает blob URLs родителя (ресурсы преобразованы в data URLs); Node lookup закреплён на IPv4; HTTP redirects разрешаются вручную через шлюз; новые boolean options сделаны optional для совместимости прежних API-запросов. Ошибочные промежуточные проверки не считаются результатом приёмки.

## Передача

Результат доступен на `http://localhost:8080`. Копии выдаются JSON bundle через ACL API, никогда как активный HTML origin приложения. В Compose добавлен capture-egress и persistent volume capture-cache, API читает blobs read-only. CI расширен container browser smoke. Коммит/push не выполнялись; незакоммиченная работа auth сохранена. Обязательных следующих шагов по этой задаче нет.
