CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(32) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    avatar_url TEXT,
    active_skin VARCHAR(64) NOT NULL DEFAULT 'classic',
    country_code VARCHAR(128),
    city VARCHAR(128),
    elo_rating INT NOT NULL DEFAULT 1000,
    peak_elo INT NOT NULL DEFAULT 1000,
    wins INT NOT NULL DEFAULT 0,
    losses INT NOT NULL DEFAULT 0,
    draws INT NOT NULL DEFAULT 0,
    current_streak INT NOT NULL DEFAULT 0,
    max_streak INT NOT NULL DEFAULT 0,
    experience INT NOT NULL DEFAULT 0,
    level INT NOT NULL DEFAULT 1,
    subscription_type VARCHAR(32) NOT NULL DEFAULT 'free',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS active_skin VARCHAR(64) NOT NULL DEFAULT 'classic';

ALTER TABLE users
    ALTER COLUMN country_code TYPE VARCHAR(128),
    ALTER COLUMN city TYPE VARCHAR(128);

CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sudoku_puzzles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    difficulty VARCHAR(32) NOT NULL,
    seed TEXT NOT NULL,
    solution JSONB NOT NULL,
    initial_board JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sudoku_puzzles_seed ON sudoku_puzzles(seed);

CREATE TABLE IF NOT EXISTS daily_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    challenge_date DATE UNIQUE NOT NULL,
    puzzle_id UUID NOT NULL REFERENCES sudoku_puzzles(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS daily_challenge_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    challenge_id UUID NOT NULL REFERENCES daily_challenges(id),
    completion_time_ms BIGINT NOT NULL,
    mistakes_count INT NOT NULL DEFAULT 0,
    hints_used INT NOT NULL DEFAULT 0,
    score INT NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, challenge_id)
);

CREATE TABLE IF NOT EXISTS matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player1_id UUID NOT NULL REFERENCES users(id),
    player2_id UUID NOT NULL REFERENCES users(id),
    puzzle_id UUID NOT NULL REFERENCES sudoku_puzzles(id),
    winner_id UUID REFERENCES users(id),
    player1_elo_before INT NOT NULL,
    player1_elo_after INT NOT NULL,
    player2_elo_before INT NOT NULL,
    player2_elo_after INT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    match_duration_ms BIGINT,
    status VARCHAR(32) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS match_moves (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    player_id UUID NOT NULL REFERENCES users(id),
    row_index INT NOT NULL,
    col_index INT NOT NULL,
    value INT NOT NULL,
    is_correct BOOLEAN NOT NULL,
    move_number INT NOT NULL,
    time_from_start_ms BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS match_analysis (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id UUID UNIQUE NOT NULL REFERENCES matches(id),
    analysis JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS skins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) NOT NULL,
    preview_url TEXT,
    price_usd NUMERIC(10,2) NOT NULL DEFAULT 0,
    is_premium BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_skins (
    user_id UUID NOT NULL REFERENCES users(id),
    skin_id UUID NOT NULL REFERENCES skins(id),
    purchased_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id, skin_id)
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    stripe_customer_id TEXT,
    stripe_subscription_id TEXT,
    status VARCHAR(32) NOT NULL,
    started_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    stripe_payment_intent_id TEXT,
    amount_usd NUMERIC(10,2),
    currency VARCHAR(8),
    status VARCHAR(32),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS achievements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(64) UNIQUE NOT NULL,
    title VARCHAR(128),
    description TEXT
);

CREATE TABLE IF NOT EXISTS user_achievements (
    user_id UUID NOT NULL REFERENCES users(id),
    achievement_id UUID NOT NULL REFERENCES achievements(id),
    unlocked_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY(user_id, achievement_id)
);

INSERT INTO sudoku_puzzles (difficulty, seed, solution, initial_board)
SELECT
    'medium',
    'daily-seed-001',
    '[[5,3,4,6,7,8,9,1,2],[6,7,2,1,9,5,3,4,8],[1,9,8,3,4,2,5,6,7],[8,5,9,7,6,1,4,2,3],[4,2,6,8,5,3,7,9,1],[7,1,3,9,2,4,8,5,6],[9,6,1,5,3,7,2,8,4],[2,8,7,4,1,9,6,3,5],[3,4,5,2,8,6,1,7,9]]'::jsonb,
    '[[5,3,0,0,7,0,0,0,0],[6,0,0,1,9,5,0,0,0],[0,9,8,0,0,0,0,6,0],[8,0,0,0,6,0,0,0,3],[4,0,0,8,0,3,0,0,1],[7,0,0,0,2,0,0,0,6],[0,6,0,0,0,0,2,8,0],[0,0,0,4,1,9,0,0,5],[0,0,0,0,8,0,0,7,9]]'::jsonb
WHERE NOT EXISTS (SELECT 1 FROM sudoku_puzzles);

INSERT INTO skins (name, preview_url, price_usd, is_premium)
SELECT 'Classic', '', 0, FALSE
WHERE NOT EXISTS (SELECT 1 FROM skins WHERE name = 'Classic');

INSERT INTO skins (name, preview_url, price_usd, is_premium)
SELECT 'Ember', '', 1.99, FALSE
WHERE NOT EXISTS (SELECT 1 FROM skins WHERE name = 'Ember');

INSERT INTO skins (name, preview_url, price_usd, is_premium)
SELECT 'Forest', '', 1.99, FALSE
WHERE NOT EXISTS (SELECT 1 FROM skins WHERE name = 'Forest');

INSERT INTO skins (name, preview_url, price_usd, is_premium)
SELECT 'Midnight', '', 2.99, TRUE
WHERE NOT EXISTS (SELECT 1 FROM skins WHERE name = 'Midnight');

INSERT INTO skins (name, preview_url, price_usd, is_premium)
SELECT 'Solar', '', 2.49, TRUE
WHERE NOT EXISTS (SELECT 1 FROM skins WHERE name = 'Solar');

INSERT INTO skins (name, preview_url, price_usd, is_premium)
SELECT 'Wave', '', 1.49, FALSE
WHERE NOT EXISTS (SELECT 1 FROM skins WHERE name = 'Wave');
