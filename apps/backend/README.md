# Sudoku Backend

MVP backend for a competitive Sudoku platform.

## Stack

- Go
- PostgreSQL
- Redis
- WebSocket

## Run

```bash
cd /Users/ila/dev/pet/sudoku/apps/backend
go run ./cmd/api
```

OpenAPI spec:

```bash
curl http://localhost:8080/openapi.yaml
```

## Environment

```bash
HTTP_ADDR=:8080
DATABASE_URL=postgresql://postgres:changeme@localhost:5432/appdb?sslmode=disable
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
JWT_SECRET=dev-secret-change-me
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=720h
SHUTDOWN_TIMEOUT=10s
MATCHMAKING_WINDOW=45s
```

## Implemented MVP Areas

- Auth: register, login, refresh, logout
- Users: me, update profile, public profile
- Matchmaking: join and leave queue with Redis
- Realtime matches: WebSocket transport and live move handling
- Elo rating: applied on match finish
- Replay: move history from `match_moves`
- AI analysis: generated from replay data and cached in PostgreSQL
- Daily challenge: current puzzle, submit result, leaderboard
- Leaderboards: global, by country, by city
- Cosmetics: skin list and purchase ownership
- Subscriptions and payments: stub endpoints ready for Stripe wiring

## Routes

- `GET /healthz`
- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /users/me`
- `PATCH /users/me`
- `GET /users/{id}`
- `POST /matchmaking/join`
- `POST /matchmaking/leave`
- `GET /matches/{id}`
- `GET /matches/history`
- `GET /matches/{id}/replay`
- `GET /matches/{id}/analysis`
- `GET /daily/`
- `POST /daily/submit`
- `GET /daily/leaderboard`
- `GET /leaderboards/global`
- `GET /leaderboards/countries`
- `GET /leaderboards/countries/{country}`
- `GET /leaderboards/cities`
- `GET /leaderboards/cities/{city}`
- `GET /skins/`
- `POST /skins/purchase`
- `GET /users/me/skins`
- `GET /subscription/me`
- `POST /subscription/cancel`
- `GET /ws?token=<access_token>&matchId=<match_id>`

## Notes

- Schema bootstrap is automatic on startup from `internal/platform/storage/schema.sql`.
- One demo puzzle and one default skin are seeded automatically.
- Live match state is in-memory plus Redis cache, so the current matchmaking/runtime is best suited for a single backend instance MVP.
