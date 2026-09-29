# AiDe Bot

MAX-бот на Go, который передаёт сообщения в AiDe Backend API.

## Запуск

Нужны Docker Compose и токен бота MAX:

```bash
cp .env.example .env
```

В `.env` заполните `MAX_BOT_TOKEN` и запустите бота:

```bash
docker compose up --build
```

## API для жюри

- `DATA-API.yaml` — сценарий автоматической проверки по стандарту DATA-API 1.0.
- `docs/backend-api.yaml` — OpenAPI-контракт запросов и ответов backend.

Остальные параметры запуска уже заданы в `.env.example`.
