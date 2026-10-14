FROM node:24.21.0-bookworm-slim@sha256:d6aa754f16b3197301076f047b5def2f02ea1dbbc2ca920407d46d7ec7f87b20 AS build
RUN npm install -g pnpm@11.19.0
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY capture/package.json capture/package.json
RUN pnpm install --frozen-lockfile
COPY tsconfig.base.json ./
COPY capture capture
RUN pnpm --filter @saveboard/capture build

# Exact match for the installed Playwright package. Browser has no direct egress.
FROM mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27
COPY --from=build /usr/local/bin/node /usr/local/bin/node
WORKDIR /app
RUN mkdir /cache && chown pwuser:pwuser /cache
COPY --from=build --chown=pwuser:pwuser /src/node_modules /app/node_modules
COPY --from=build --chown=pwuser:pwuser /src/capture /app/capture
USER pwuser
EXPOSE 3001
CMD ["node", "capture/dist/main.js"]
