# Sudoku Clash

Sudoku Clash is a competitive mobile-first Sudoku platform where players race on the same board in real time, climb ranked ladders, solve a shared daily challenge, and build a visible identity through skins and profile customization.

## What We Built

We built a full-stack competitive Sudoku product with:

- `Ranked` real-time PvP Sudoku matches
- `Daily Challenge` with one shared Sudoku per day for all players
- `Daily Leaders` leaderboard for today’s exact board
- `Global / Country / City Ranks`
- `Replay + AI-style post-game analysis`
- `Stripe` checkout flow for paid skins
- `Skin inventory and equip flow` for player individuality
- `Auth, profiles, Elo, replay storage, WebSocket gameplay, leaderboards`

This is not just a Sudoku solver. It is a session-based competitive experience designed for fast mobile play, progression, identity, and replayable daily engagement.

## Who It Is For

Sudoku Clash is built for:

- players who already enjoy Sudoku and want competition, not only solo solving
- mobile users who want quick daily and ranked sessions
- users who like visible progression through `Elo`, ranks, streaks, levels, and cosmetics
- communities that benefit from shared daily boards and public leaderboards

## Why It Is Valuable

Sudoku Clash turns a traditionally solo logic puzzle into a social competitive game loop:

- `Ranked` adds tension, pressure, and replayability
- `Daily` creates a shared ritual and retention mechanic
- `Leaderboards` give players public status
- `AI analysis` helps users understand mistakes and improve
- `Skins` and `Stripe` add personalization and monetization

That combination makes the product useful for both players and operators:

- players get a more exciting Sudoku experience
- the platform gets retention, progression, and monetization layers

## Core Features

### Ranked

Players queue into live Sudoku matches and race on the same puzzle. Matchmaking, Elo updates, timers, progress tracking, and post-game results are all handled by the platform.

### Daily Challenge

Every day all players receive the same daily Sudoku. Once a user completes it, the board is closed for the day and their run is locked into the `Daily Leaders` board.

### Leaderboards

The product includes:

- global ranks
- country ranks
- city ranks
- daily leaders for the current shared puzzle

### AI Game Analysis

Every ranked game stores move replay data and generates post-game analysis based on:

- move timing
- mistakes
- repeated cells
- pressure moments
- correct streaks
- recovery after mistakes

This gives players actionable feedback on how to improve.

### Skins and Individuality

Players can unlock and equip skins that change the visual look of their experience. Paid skins are integrated with Stripe checkout so cosmetics can be monetized cleanly.

## Tech Stack

### Application

- `Go` backend
- `React` frontend
- `Tailwind CSS`
- `WebSocket` for live matches

### Data and State

- `PostgreSQL` for persistent app data
- `Redis` for matchmaking and live state helpers

### Payments and Infra

- `Stripe` for skin purchases
- `Docker` and `Docker Compose` for full local / server deployment
- `Nginx` as the web entrypoint and reverse proxy
- `Cloudflare Tunnel` support for publishing the app under `sudoku.endfieldhq.com`
- `Kubernetes manifests` for core infra pieces such as PostgreSQL and Redis under [`infra/`](/Users/ila/dev/pet/sudoku/infra)

## Repository Structure

```txt
apps/
  backend/   -> Go API, WebSocket server, auth, matchmaking, ratings, daily, payments
  frontend/  -> React mobile-first client

docker/
  -> docker-compose stack, nginx, cloudflared notes, DB migration helpers

infra/
  -> Kubernetes manifests for core services
```

## How To Run

### 1. Clone the repository

```bash
git clone git@github.com:MrDrtyui/SudokuClash.git
cd SudokuClash
```

### 2. Prepare the deployment environment

The project already includes a root `Makefile` and a full Docker deployment stack.

Bootstrap the deployment env:

```bash
make env
```

This creates and auto-fills:

```bash
docker/.env
```

For a normal local launch, you do not need to manually edit `.env`.
The bootstrap script generates working local defaults and local secrets automatically, including:

- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- local `FRONTEND_URL`
- local Docker / API / DB defaults

Optional external secrets can still be injected automatically from the machine environment if they already exist:

- `STRIPE_SECRET_KEY`
- `STRIPE_WEBHOOK_SECRET`
- `CLOUDFLARE_TUNNEL_TOKEN`

If you want DB migration from an existing source database, make sure `SOURCE_DATABASE_URL` is correct in `docker/.env`.

### 3. Run the full application

```bash
make run
```

What `make run` does:

1. ensures `docker/.env` exists
2. auto-generates local env values if they are missing
3. creates a dump from the current source database if auto-dump is enabled
4. imports that dump into the Docker Postgres volume if auto-migrate is enabled
5. starts the full stack with Docker Compose

### 4. Open the app

By default the application will be available at:

```txt
http://localhost:8088
```

The stack includes:

- frontend served by `nginx`
- backend Go API behind `/api`
- WebSocket endpoint behind `/ws`
- PostgreSQL
- Redis

## Useful Commands

```bash
make env
make run
make stop
make down
make logs
make ps
make export-db
make migrate-db
```

## Summary

Sudoku Clash is a competitive, mobile-first Sudoku platform with ranked play, daily shared puzzles, daily leaders, public ranks, AI-style post-game analysis, and Stripe-powered skin purchases. The repository already contains both the product code and the infrastructure layer needed to run it locally or deploy it as a full application stack.
