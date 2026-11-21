# sweeper

Data retention service for PurgeBot. Deletes a server's rows in Postgres and its purge keys in Redis once the bot has been out of it for more than 30 days.

## Responsibilities

- Lists the servers the bot is in every hour and records when a server with stored data is no longer listed
- Clears that record when the server is listed again or Discord shows the bot is still in it; the gateway also clears it on a genuine join
- Marks and deletes nothing when the list failed part way or holds under 90% of Discord's approximate guild count
- Confirms each server due for deletion with Discord, at most 100 per sweep and oldest first, then deletes its purge keys in Redis and every row in Postgres, and rewrites the Redis AOF
- `-once` runs a single sweep and exits: `docker compose run --rm sweeper -once`

## Configuration

All configuration is loaded from environment variables (see `.env.example` in the docker repo).

| Variable            | Description                                       |
| ------------------- | ------------------------------------------------- |
| `DISCORD_TOKEN`     | Bot token (required)                              |
| `DATABASE_HOST`     | PostgreSQL host (default `localhost`)             |
| `DATABASE_PORT`     | PostgreSQL port (default `5432`)                  |
| `DATABASE_NAME`     | Database name (default `purgebot`)                |
| `DATABASE_USER`     | Database user (default `purgebot`)                |
| `DATABASE_PASSWORD` | Database password                                 |
| `REDIS_ADDR`        | Redis address (default `localhost:6379`)          |
| `REDIS_PASSWORD`    | Redis password                                    |
| `REDIS_DB`          | Redis database index (default `0`)                |
| `SENTRY_DSN`        | Sentry-compatible error reporting (optional)      |
| `LOG_LEVEL`         | `debug`, `info`, `warn`, `error` (default `info`) |
| `LOG_JSON`          | `true` for JSON log output                        |
