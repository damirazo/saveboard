# Saveboard frontend

Самостоятельный React/TypeScript SPA workspace. Весь UI, API-клиент, тесты, конфигурации и зависимости находятся здесь; Go и исходники backend для сборки не нужны.

Требования: Node.js 24, pnpm 11.19.0. Из этого каталога:

```sh
pnpm install --frozen-lockfile
pnpm check
pnpm dev
pnpm exec playwright install chromium
pnpm test:e2e
docker build -t saveboard/web:dev .
```

Vite слушает `127.0.0.1:5173` и проксирует `/api` и `/auth` к `127.0.0.1:8080`. Браузерный smoke использует fixtures; проверка живого стека находится в общих deployment-сценариях. Для установленного Chrome: задать `PLAYWRIGHT_CHANNEL=chrome` перед `pnpm test:e2e`.

`api/openapi.json` — входной версионированный снимок API. После его получения от бэкенда выполнить `pnpm api:generate`; generated TypeScript находится в `packages/api-client/src/schema.d.ts`. Команда не запускает Go и не читает соседний каталог. Снимок и generated types хранить вместе в Git.

Для выделения в репозиторий достаточно перенести каталог целиком. Docker build использует только этот каталог. Nginx-конфигурация рассчитана на deployment с доступными DNS-именами `api` и `keycloak`; при иной топологии поменять upstream в `nginx.conf`.

## Интерфейс

[Дизайн-лист, карта экранов и правила компоновки](../docs/design/README.md) · [Живой каталог компонентов](http://127.0.0.1:5173/ui-kit) · [Состав UI-kit](src/components/README.md).
