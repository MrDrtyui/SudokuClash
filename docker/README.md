# Sudoku Deploy Stack

This directory contains a full containerized deployment stack for the Sudoku app:

- `postgres` with a persistent Docker volume
- `redis` with AOF persistence
- `backend` (Go API / WS)
- `nginx` serving the built frontend and reverse proxying `/api` and `/ws`
- optional `cloudflared` profile for `sudoku.endfieldhq.com`

## Layout

- `docker-compose.yml`
- `.env.example`
- `backend.Dockerfile`
- `nginx.Dockerfile`
- `nginx/nginx.conf`
- `cloudflared/README.md`
- `scripts/export-current-db.sh`
- `scripts/import-dump.sh`
- `scripts/migrate-current-db-to-volume.sh`

## 1. Prepare env

```bash
cd /Users/ila/dev/pet/sudoku
make env
```

This creates `docker/.env` and auto-fills local defaults plus generated secrets for local use.

You only need manual values if you want external integrations such as:

- `CLOUDFLARE_TUNNEL_TOKEN`
- `STRIPE_SECRET_KEY`
- `STRIPE_WEBHOOK_SECRET`
- a custom `SOURCE_DATABASE_URL`

## 2. Migrate the current database into the new postgres volume

If your current DB is still exposed on `localhost:5432` (for example through the existing port-forward), `make run` can already export and re-import it automatically.

If you want to run the migration step manually:

```bash
cd /Users/ila/dev/pet/sudoku
make migrate-db
```

This will:

1. export the current DB into `docker/backups/*.dump`
2. start the new `postgres` service
3. restore the dump into the new Docker volume

## 3. Start the stack

```bash
cd /Users/ila/dev/pet/sudoku
make run
```

`make run` will:

1. bootstrap `docker/.env`
2. dump the source DB if `AUTO_DUMP_ON_RUN=true`
3. import the latest dump if `AUTO_MIGRATE_ON_RUN=true`
4. start the full compose stack

Other useful commands:

```bash
make validate
make logs
make ps
make stop
make down
make tunnel
make migrate-db
```

Local entrypoint:

- frontend + nginx: `http://localhost:${NGINX_HTTP_PORT:-8088}`
- backend API through nginx: `http://localhost:${NGINX_HTTP_PORT:-8088}/api`

The raw postgres service is published on `${POSTGRES_PORT:-5433}` to avoid colliding with an existing local `5432`.

## 4. Cloudflare tunnel for sudoku.endfieldhq.com

Recommended approach: dashboard-managed Cloudflare Tunnel.

1. Create a tunnel in Cloudflare Zero Trust.
2. Add a public hostname:
   - Hostname: `sudoku.endfieldhq.com`
   - Service type: `HTTP`
   - URL: `http://nginx:80`
3. Put the generated token into `.env`:

```bash
CLOUDFLARE_TUNNEL_TOKEN=...
```

4. Start the tunnel profile:

```bash
docker compose --profile tunnel up -d
```

## Notes

- The current backend runtime is still single-instance for live match state, so keep only one `backend` replica.
- If your current source DB is not actually reachable on `host.docker.internal:5432`, update `SOURCE_DATABASE_URL` in `.env` before running the migration scripts.
