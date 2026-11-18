# sweeper

Data retention service for PurgeBot. Deletes everything PurgeBot stores about a server once the bot has been out of it for more than 30 days.

## Responsibilities

- Lists the servers the bot is in every hour and records when a server with stored data is no longer listed
- Clears that record when the server is listed again; the gateway also clears it on a genuine join
- Refuses to act on a list that failed part way or holds under 90% of Discord's approximate guild count
- Confirms each server due for deletion with Discord, then deletes its purge keys in Redis and every row in Postgres, and rewrites the Redis AOF
- `-once` runs a single sweep and exits: `docker compose run --rm sweeper -once`

## Configuration

All configuration is loaded from environment variables (see `.env.example` in the docker repo).

| Variable         | Description                                  |
| ---------------- | -------------------------------------------- |
| `DISCORD_TOKEN`  | Bot token                                    |
| `DATABASE_*`     | PostgreSQL connection                        |
| `REDIS_ADDR`     | Redis address                                |
| `REDIS_PASSWORD` | Redis password                               |
| `REDIS_DB`       | Redis database index                         |
| `SENTRY_DSN`     | Sentry-compatible error reporting (optional) |
| `LOG_LEVEL`      | `debug`, `info`, `warn`, `error`             |
| `LOG_JSON`       | `true` for JSON log output                   |
