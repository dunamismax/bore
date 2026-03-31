FROM oven/bun:1.3.10

WORKDIR /app

COPY package.json bun.lock tsconfig.base.json biome.json ./
COPY scripts ./scripts
COPY apps ./apps
COPY packages ./packages

RUN bun install --frozen-lockfile
RUN bun run build

WORKDIR /app/apps/api

CMD ["bun", "run", "start"]
