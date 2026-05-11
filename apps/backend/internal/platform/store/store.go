package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"sudoku-backend/internal/domain"
	"sudoku-backend/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
}

type MatchSubmission struct {
	Row   int
	Col   int
	Value int
}

func New(db *pgxpool.Pool, redisClient *redis.Client) *Store {
	return &Store{DB: db, Redis: redisClient}
}

func (s *Store) CreateUser(ctx context.Context, username, email, passwordHash string) (domain.User, error) {
	row := s.DB.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, username, email, avatar_url, active_skin, country_code, city, elo_rating, peak_elo, wins, losses, draws, current_streak, max_streak, experience, level, subscription_type, created_at
	`, username, email, passwordHash)
	return scanUser(row)
}

func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, string, error) {
	row := s.DB.QueryRow(ctx, `
		SELECT id, username, email, avatar_url, active_skin, country_code, city, elo_rating, peak_elo, wins, losses, draws, current_streak, max_streak, experience, level, subscription_type, created_at, password_hash
		FROM users WHERE email = $1
	`, email)
	user, err := scanUserWithPassword(row)
	if err != nil {
		return domain.User{}, "", err
	}
	return user.user, user.passwordHash, nil
}

type userWithPassword struct {
	user         domain.User
	passwordHash string
}

func (s *Store) UserByID(ctx context.Context, userID string) (domain.User, error) {
	row := s.DB.QueryRow(ctx, `
		SELECT id, username, email, avatar_url, active_skin, country_code, city, elo_rating, peak_elo, wins, losses, draws, current_streak, max_streak, experience, level, subscription_type, created_at
		FROM users WHERE id = $1
	`, userID)
	return scanUser(row)
}

func (s *Store) UpdateUserProfile(ctx context.Context, userID, avatarURL, activeSkin, countryCode, city string) (domain.User, error) {
	row := s.DB.QueryRow(ctx, `
		UPDATE users
		SET avatar_url = COALESCE(NULLIF($2, ''), avatar_url),
		    active_skin = COALESCE(NULLIF($3, ''), active_skin),
		    country_code = COALESCE(NULLIF($4, ''), country_code),
		    city = COALESCE(NULLIF($5, ''), city),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, username, email, avatar_url, active_skin, country_code, city, elo_rating, peak_elo, wins, losses, draws, current_streak, max_streak, experience, level, subscription_type, created_at
	`, userID, avatarURL, activeSkin, countryCode, city)
	return scanUser(row)
}

func (s *Store) CreateSession(ctx context.Context, userID, refreshToken, ipAddress, userAgent string, expiresAt time.Time) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO user_sessions (user_id, refresh_token, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, refreshToken, ipAddress, userAgent, expiresAt)
	return err
}

func (s *Store) SessionByRefreshToken(ctx context.Context, refreshToken string) (string, time.Time, error) {
	row := s.DB.QueryRow(ctx, `SELECT user_id, expires_at FROM user_sessions WHERE refresh_token = $1`, refreshToken)
	var userID string
	var expiresAt time.Time
	err := row.Scan(&userID, &expiresAt)
	return userID, expiresAt, err
}

func (s *Store) DeleteSession(ctx context.Context, refreshToken string) error {
	_, err := s.DB.Exec(ctx, `DELETE FROM user_sessions WHERE refresh_token = $1`, refreshToken)
	return err
}

func (s *Store) RandomPuzzle(ctx context.Context, averageElo int) (domain.Puzzle, error) {
	seed := service.RankedSeed(fmt.Sprintf("%d-%s", time.Now().UnixNano(), NewID()))
	difficulty := service.RankedDifficultyForElo(seed, averageElo)
	return s.ensureGeneratedPuzzle(ctx, seed, difficulty)
}

func (s *Store) PuzzleByID(ctx context.Context, puzzleID string) (domain.Puzzle, error) {
	row := s.DB.QueryRow(ctx, `
		SELECT id, difficulty, seed, solution, initial_board, created_at
		FROM sudoku_puzzles WHERE id = $1
	`, puzzleID)
	return scanPuzzle(row)
}

func scanPuzzle(row pgx.Row) (domain.Puzzle, error) {
	var puzzle domain.Puzzle
	var solutionJSON, initialJSON []byte
	if err := row.Scan(&puzzle.ID, &puzzle.Difficulty, &puzzle.Seed, &solutionJSON, &initialJSON, &puzzle.CreatedAt); err != nil {
		return puzzle, err
	}
	if err := json.Unmarshal(solutionJSON, &puzzle.Solution); err != nil {
		return puzzle, err
	}
	if err := json.Unmarshal(initialJSON, &puzzle.InitialBoard); err != nil {
		return puzzle, err
	}
	return puzzle, nil
}

func (s *Store) CreateMatch(ctx context.Context, player1ID, player2ID string, puzzle domain.Puzzle, player1Elo, player2Elo int) (domain.Match, error) {
	row := s.DB.QueryRow(ctx, `
		INSERT INTO matches (
			player1_id, player2_id, puzzle_id, player1_elo_before, player1_elo_after,
			player2_elo_before, player2_elo_after, started_at, status
		) VALUES ($1, $2, $3, $4, $4, $5, $5, NOW(), 'active')
		RETURNING id, player1_id, player2_id, puzzle_id, winner_id, player1_elo_before, player1_elo_after, player2_elo_before, player2_elo_after, started_at, ended_at, match_duration_ms, status, created_at
	`, player1ID, player2ID, puzzle.ID, player1Elo, player2Elo)
	return scanMatch(row)
}

func (s *Store) MatchByID(ctx context.Context, matchID string) (domain.Match, error) {
	row := s.DB.QueryRow(ctx, `
		SELECT id, player1_id, player2_id, puzzle_id, winner_id, player1_elo_before, player1_elo_after, player2_elo_before, player2_elo_after, started_at, ended_at, match_duration_ms, status, created_at
		FROM matches WHERE id = $1
	`, matchID)
	return scanMatch(row)
}

func (s *Store) MatchHistory(ctx context.Context, userID string) ([]domain.Match, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, player1_id, player2_id, puzzle_id, winner_id, player1_elo_before, player1_elo_after, player2_elo_before, player2_elo_after, started_at, ended_at, match_duration_ms, status, created_at
		FROM matches
		WHERE player1_id = $1 OR player2_id = $1
		ORDER BY created_at DESC
		LIMIT 20
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []domain.Match
	for rows.Next() {
		match, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		matches = append(matches, match)
	}
	return matches, rows.Err()
}

func (s *Store) RecordMove(ctx context.Context, matchID, playerID string, move MatchSubmission, moveNumber int, elapsedMS int64, isCorrect bool) (domain.MatchMove, error) {
	row := s.DB.QueryRow(ctx, `
		INSERT INTO match_moves (match_id, player_id, row_index, col_index, value, is_correct, move_number, time_from_start_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, match_id, player_id, row_index, col_index, value, is_correct, move_number, time_from_start_ms, created_at
	`, matchID, playerID, move.Row, move.Col, move.Value, isCorrect, moveNumber, elapsedMS)
	var saved domain.MatchMove
	err := row.Scan(&saved.ID, &saved.MatchID, &saved.PlayerID, &saved.RowIndex, &saved.ColIndex, &saved.Value, &saved.IsCorrect, &saved.MoveNumber, &saved.TimeFromStartMS, &saved.CreatedAt)
	return saved, err
}

func (s *Store) MatchMoves(ctx context.Context, matchID string) ([]domain.MatchMove, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, match_id, player_id, row_index, col_index, value, is_correct, move_number, time_from_start_ms, created_at
		FROM match_moves WHERE match_id = $1 ORDER BY move_number ASC
	`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var moves []domain.MatchMove
	for rows.Next() {
		var move domain.MatchMove
		if err := rows.Scan(&move.ID, &move.MatchID, &move.PlayerID, &move.RowIndex, &move.ColIndex, &move.Value, &move.IsCorrect, &move.MoveNumber, &move.TimeFromStartMS, &move.CreatedAt); err != nil {
			return nil, err
		}
		moves = append(moves, move)
	}
	return moves, rows.Err()
}

func (s *Store) FinishMatch(ctx context.Context, match domain.Match, winnerID *string, durationMS int64) (domain.Match, error) {
	player1After := match.Player1EloBefore
	player2After := match.Player2EloBefore
	if winnerID != nil {
		switch *winnerID {
		case match.Player1ID:
			player1After = service.CalculateElo(match.Player1EloBefore, match.Player2EloBefore, 1)
			player2After = service.CalculateElo(match.Player2EloBefore, match.Player1EloBefore, 0)
		case match.Player2ID:
			player1After = service.CalculateElo(match.Player1EloBefore, match.Player2EloBefore, 0)
			player2After = service.CalculateElo(match.Player2EloBefore, match.Player1EloBefore, 1)
		}
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.Match{}, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		UPDATE matches
		SET winner_id = $2,
		    player1_elo_after = $3,
		    player2_elo_after = $4,
		    ended_at = NOW(),
		    match_duration_ms = $5,
		    status = 'finished'
		WHERE id = $1
		RETURNING id, player1_id, player2_id, puzzle_id, winner_id, player1_elo_before, player1_elo_after, player2_elo_before, player2_elo_after, started_at, ended_at, match_duration_ms, status, created_at
	`, match.ID, winnerID, player1After, player2After, durationMS)
	finished, err := scanMatch(row)
	if err != nil {
		return domain.Match{}, err
	}

	if winnerID == nil {
		_, err = tx.Exec(ctx, `
			UPDATE users
			SET draws = draws + 1
			WHERE id = $1 OR id = $2
		`, match.Player1ID, match.Player2ID)
	} else {
		loserID := match.Player1ID
		winnerAfter := player2After
		loserAfter := player1After
		if *winnerID == match.Player1ID {
			loserID = match.Player2ID
			winnerAfter = player1After
			loserAfter = player2After
		}
		_, err = tx.Exec(ctx, `
			UPDATE users
			SET elo_rating = CASE WHEN id = $1 THEN $3 WHEN id = $2 THEN $4 ELSE elo_rating END,
			    peak_elo = CASE WHEN id = $1 THEN GREATEST(peak_elo, $3) WHEN id = $2 THEN GREATEST(peak_elo, $4) ELSE peak_elo END,
			    wins = CASE WHEN id = $1 THEN wins + 1 ELSE wins END,
			    losses = CASE WHEN id = $2 THEN losses + 1 ELSE losses END,
			    current_streak = CASE WHEN id = $1 THEN current_streak + 1 WHEN id = $2 THEN 0 ELSE current_streak END,
			    max_streak = CASE WHEN id = $1 THEN GREATEST(max_streak, current_streak + 1) ELSE max_streak END,
			    experience = CASE WHEN id = $1 THEN experience + 30 WHEN id = $2 THEN experience + 10 ELSE experience END,
			    level = GREATEST(1, ((experience + CASE WHEN id = $1 THEN 30 WHEN id = $2 THEN 10 ELSE 0 END) / 100) + 1)
			WHERE id = $1 OR id = $2
		`, *winnerID, loserID, winnerAfter, loserAfter)
	}
	if err != nil {
		return domain.Match{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Match{}, err
	}
	return finished, nil
}

func (s *Store) SaveAnalysis(ctx context.Context, matchID string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(ctx, `
		INSERT INTO match_analysis (match_id, analysis)
		VALUES ($1, $2)
		ON CONFLICT (match_id) DO UPDATE SET analysis = EXCLUDED.analysis, created_at = NOW()
	`, matchID, raw)
	return err
}

func (s *Store) AnalysisByMatchID(ctx context.Context, matchID string) (domain.Analysis, error) {
	row := s.DB.QueryRow(ctx, `SELECT match_id, analysis, created_at FROM match_analysis WHERE match_id = $1`, matchID)
	var analysis domain.Analysis
	var raw []byte
	if err := row.Scan(&analysis.MatchID, &raw, &analysis.CreatedAt); err != nil {
		return analysis, err
	}
	err := json.Unmarshal(raw, &analysis.Analysis)
	return analysis, err
}

func (s *Store) EnsureDailyChallenge(ctx context.Context, date string) (domain.DailyChallenge, domain.Puzzle, error) {
	seed := service.DailySeed(date)
	difficulty := service.DailyDifficulty(seed)
	puzzle, err := s.ensureGeneratedPuzzle(ctx, seed, difficulty)
	if err != nil {
		return domain.DailyChallenge{}, domain.Puzzle{}, err
	}
	row := s.DB.QueryRow(ctx, `
		INSERT INTO daily_challenges (challenge_date, puzzle_id)
		VALUES ($1, $2)
		ON CONFLICT (challenge_date) DO UPDATE SET puzzle_id = daily_challenges.puzzle_id
		RETURNING id, challenge_date::text, puzzle_id, created_at
	`, date, puzzle.ID)
	var challenge domain.DailyChallenge
	err = row.Scan(&challenge.ID, &challenge.ChallengeDate, &challenge.PuzzleID, &challenge.CreatedAt)
	return challenge, puzzle, err
}

func (s *Store) ensureGeneratedPuzzle(ctx context.Context, seed, difficulty string) (domain.Puzzle, error) {
	row := s.DB.QueryRow(ctx, `
		SELECT id, difficulty, seed, solution, initial_board, created_at
		FROM sudoku_puzzles WHERE seed = $1
	`, seed)
	puzzle, err := scanPuzzle(row)
	if err == nil {
		return puzzle, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Puzzle{}, err
	}

	generated := service.GeneratePuzzle(seed, difficulty)
	solutionJSON, err := service.EncodeBoard(generated.Solution)
	if err != nil {
		return domain.Puzzle{}, err
	}
	initialJSON, err := service.EncodeBoard(generated.InitialBoard)
	if err != nil {
		return domain.Puzzle{}, err
	}

	insertRow := s.DB.QueryRow(ctx, `
		INSERT INTO sudoku_puzzles (difficulty, seed, solution, initial_board)
		VALUES ($1, $2, $3::jsonb, $4::jsonb)
		ON CONFLICT (seed) DO UPDATE SET difficulty = EXCLUDED.difficulty
		RETURNING id, difficulty, seed, solution, initial_board, created_at
	`, generated.Difficulty, generated.Seed, solutionJSON, initialJSON)
	return scanPuzzle(insertRow)
}

func (s *Store) UpsertDailyResult(ctx context.Context, userID, challengeID string, completionTimeMS int64, mistakes, hints, score int, completed bool) (domain.DailyChallengeResult, error) {
	row := s.DB.QueryRow(ctx, `
		INSERT INTO daily_challenge_results (user_id, challenge_id, completion_time_ms, mistakes_count, hints_used, score, completed)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, challenge_id)
		DO UPDATE SET
			completion_time_ms = EXCLUDED.completion_time_ms,
			mistakes_count = EXCLUDED.mistakes_count,
			hints_used = EXCLUDED.hints_used,
			score = EXCLUDED.score,
			completed = EXCLUDED.completed
		RETURNING id, user_id, challenge_id, completion_time_ms, mistakes_count, hints_used, score, completed, created_at
	`, userID, challengeID, completionTimeMS, mistakes, hints, score, completed)
	var result domain.DailyChallengeResult
	err := row.Scan(&result.ID, &result.UserID, &result.ChallengeID, &result.CompletionTimeMS, &result.MistakesCount, &result.HintsUsed, &result.Score, &result.Completed, &result.CreatedAt)
	return result, err
}

func (s *Store) DailyResultByUserChallenge(ctx context.Context, userID, challengeID string) (domain.DailyChallengeResult, error) {
	row := s.DB.QueryRow(ctx, `
		SELECT id, user_id, challenge_id, completion_time_ms, mistakes_count, hints_used, score, completed, created_at
		FROM daily_challenge_results
		WHERE user_id = $1 AND challenge_id = $2
	`, userID, challengeID)
	var result domain.DailyChallengeResult
	err := row.Scan(&result.ID, &result.UserID, &result.ChallengeID, &result.CompletionTimeMS, &result.MistakesCount, &result.HintsUsed, &result.Score, &result.Completed, &result.CreatedAt)
	return result, err
}

func (s *Store) DailyLeaderboard(ctx context.Context, challengeID string) ([]domain.DailyChallengeResult, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, user_id, challenge_id, completion_time_ms, mistakes_count, hints_used, score, completed, created_at
		FROM daily_challenge_results
		WHERE challenge_id = $1 AND completed = TRUE
		ORDER BY score DESC, completion_time_ms ASC
		LIMIT 50
	`, challengeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.DailyChallengeResult
	for rows.Next() {
		var r domain.DailyChallengeResult
		if err := rows.Scan(&r.ID, &r.UserID, &r.ChallengeID, &r.CompletionTimeMS, &r.MistakesCount, &r.HintsUsed, &r.Score, &r.Completed, &r.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) GlobalLeaderboard(ctx context.Context, limit int) ([]domain.User, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, username, email, avatar_url, active_skin, country_code, city, elo_rating, peak_elo, wins, losses, draws, current_streak, max_streak, experience, level, subscription_type, created_at
		FROM users ORDER BY elo_rating DESC, wins DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (s *Store) CountryLeaderboard(ctx context.Context, country string, limit int) ([]domain.User, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, username, email, avatar_url, active_skin, country_code, city, elo_rating, peak_elo, wins, losses, draws, current_streak, max_streak, experience, level, subscription_type, created_at
		FROM users WHERE country_code = $1 ORDER BY elo_rating DESC, wins DESC LIMIT $2
	`, country, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (s *Store) CityLeaderboard(ctx context.Context, city string, limit int) ([]domain.User, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT id, username, email, avatar_url, active_skin, country_code, city, elo_rating, peak_elo, wins, losses, draws, current_streak, max_streak, experience, level, subscription_type, created_at
		FROM users WHERE city = $1 ORDER BY elo_rating DESC, wins DESC LIMIT $2
	`, city, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (s *Store) DistinctCountries(ctx context.Context) ([]string, error) {
	rows, err := s.DB.Query(ctx, `SELECT DISTINCT country_code FROM users WHERE country_code IS NOT NULL AND country_code <> '' ORDER BY country_code ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) DistinctCities(ctx context.Context) ([]string, error) {
	rows, err := s.DB.Query(ctx, `SELECT DISTINCT city FROM users WHERE city IS NOT NULL AND city <> '' ORDER BY city ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) ListSkins(ctx context.Context) ([]domain.Skin, error) {
	rows, err := s.DB.Query(ctx, `SELECT id, name, preview_url, price_usd::float8, is_premium, created_at FROM skins ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var skins []domain.Skin
	for rows.Next() {
		var skin domain.Skin
		var previewURL sql.NullString
		if err := rows.Scan(&skin.ID, &skin.Name, &previewURL, &skin.PriceUSD, &skin.IsPremium, &skin.CreatedAt); err != nil {
			return nil, err
		}
		skin.PreviewURL = nullableString(previewURL)
		skins = append(skins, skin)
	}
	return skins, rows.Err()
}

func (s *Store) PurchaseSkin(ctx context.Context, userID, skinID string) error {
	_, err := s.DB.Exec(ctx, `INSERT INTO user_skins (user_id, skin_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, skinID)
	return err
}

func (s *Store) UserSkins(ctx context.Context, userID string) ([]domain.Skin, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT s.id, s.name, s.preview_url, s.price_usd::float8, s.is_premium, s.created_at
		FROM skins s JOIN user_skins us ON us.skin_id = s.id
		WHERE us.user_id = $1 ORDER BY us.purchased_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var skins []domain.Skin
	for rows.Next() {
		var skin domain.Skin
		var previewURL sql.NullString
		if err := rows.Scan(&skin.ID, &skin.Name, &previewURL, &skin.PriceUSD, &skin.IsPremium, &skin.CreatedAt); err != nil {
			return nil, err
		}
		skin.PreviewURL = nullableString(previewURL)
		skins = append(skins, skin)
	}
	return skins, rows.Err()
}

func (s *Store) SubscriptionByUser(ctx context.Context, userID string) (map[string]any, error) {
	row := s.DB.QueryRow(ctx, `
		SELECT id, status, started_at, expires_at, stripe_customer_id, stripe_subscription_id
		FROM subscriptions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1
	`, userID)
	var id, status string
	var startedAt, expiresAt *time.Time
	var customerID, subscriptionID *string
	if err := row.Scan(&id, &status, &startedAt, &expiresAt, &customerID, &subscriptionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]any{"status": "free"}, nil
		}
		return nil, err
	}
	return map[string]any{
		"id":                   id,
		"status":               status,
		"startedAt":            startedAt,
		"expiresAt":            expiresAt,
		"stripeCustomerId":     customerID,
		"stripeSubscriptionId": subscriptionID,
	}, nil
}

func (s *Store) CancelSubscription(ctx context.Context, userID string) error {
	_, err := s.DB.Exec(ctx, `UPDATE subscriptions SET status = 'cancelled' WHERE user_id = $1 AND status <> 'cancelled'`, userID)
	return err
}

func (s *Store) EnqueueMatchmaking(ctx context.Context, userID, mode string) error {
	key := fmt.Sprintf("queue:%s", mode)
	if err := s.Redis.LRem(ctx, key, 0, userID).Err(); err != nil {
		return err
	}
	return s.Redis.RPush(ctx, key, userID).Err()
}

func (s *Store) LeaveMatchmaking(ctx context.Context, userID, mode string) error {
	key := fmt.Sprintf("queue:%s", mode)
	return s.Redis.LRem(ctx, key, 0, userID).Err()
}

func (s *Store) TryPopOpponent(ctx context.Context, userID, mode string) (string, error) {
	key := fmt.Sprintf("queue:%s", mode)
	for {
		opponent, err := s.Redis.LPop(ctx, key).Result()
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if opponent == userID {
			continue
		}
		return opponent, nil
	}
}

func (s *Store) CacheMatchState(ctx context.Context, matchID string, state map[string]any, ttl time.Duration) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.Redis.Set(ctx, "match:"+matchID, raw, ttl).Err()
}

func (s *Store) MatchState(ctx context.Context, matchID string) (map[string]any, error) {
	raw, err := s.Redis.Get(ctx, "match:"+matchID).Bytes()
	if err != nil {
		return nil, err
	}
	var state map[string]any
	err = json.Unmarshal(raw, &state)
	return state, err
}

func (s *Store) SetUserActiveMatch(ctx context.Context, userID, matchID string, ttl time.Duration) error {
	return s.Redis.Set(ctx, "active_match:user:"+userID, matchID, ttl).Err()
}

func (s *Store) UserActiveMatch(ctx context.Context, userID string) (string, error) {
	value, err := s.Redis.Get(ctx, "active_match:user:"+userID).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return value, err
}

func (s *Store) ClearUserActiveMatch(ctx context.Context, userID string) error {
	return s.Redis.Del(ctx, "active_match:user:"+userID).Err()
}

func (s *Store) SetUserOnline(ctx context.Context, userID string, ttl time.Duration) error {
	return s.Redis.Set(ctx, "online:user:"+userID, "1", ttl).Err()
}

func (s *Store) RemoveUserOnline(ctx context.Context, userID string) error {
	return s.Redis.Del(ctx, "online:user:"+userID).Err()
}

func scanUser(row pgx.Row) (domain.User, error) {
	var user domain.User
	var avatarURL, activeSkin, countryCode, city sql.NullString
	err := row.Scan(
		&user.ID, &user.Username, &user.Email, &avatarURL, &activeSkin, &countryCode, &city,
		&user.EloRating, &user.PeakElo, &user.Wins, &user.Losses, &user.Draws, &user.CurrentStreak,
		&user.MaxStreak, &user.Experience, &user.Level, &user.SubscriptionType, &user.CreatedAt,
	)
	user.AvatarURL = nullableString(avatarURL)
	user.ActiveSkin = nullableString(activeSkin)
	user.CountryCode = nullableString(countryCode)
	user.City = nullableString(city)
	return user, err
}

func scanUserWithPassword(row pgx.Row) (userWithPassword, error) {
	var user domain.User
	var result userWithPassword
	var avatarURL, activeSkin, countryCode, city sql.NullString
	err := row.Scan(
		&user.ID, &user.Username, &user.Email, &avatarURL, &activeSkin, &countryCode, &city,
		&user.EloRating, &user.PeakElo, &user.Wins, &user.Losses, &user.Draws, &user.CurrentStreak,
		&user.MaxStreak, &user.Experience, &user.Level, &user.SubscriptionType, &user.CreatedAt, &result.passwordHash,
	)
	user.AvatarURL = nullableString(avatarURL)
	user.ActiveSkin = nullableString(activeSkin)
	user.CountryCode = nullableString(countryCode)
	user.City = nullableString(city)
	result.user = user
	return result, err
}

func scanUsers(rows pgx.Rows) ([]domain.User, error) {
	var users []domain.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func scanMatch(row pgx.Row) (domain.Match, error) {
	var match domain.Match
	var durationMS sql.NullInt64
	err := row.Scan(
		&match.ID, &match.Player1ID, &match.Player2ID, &match.PuzzleID, &match.WinnerID,
		&match.Player1EloBefore, &match.Player1EloAfter, &match.Player2EloBefore, &match.Player2EloAfter,
		&match.StartedAt, &match.EndedAt, &durationMS, &match.Status, &match.CreatedAt,
	)
	if durationMS.Valid {
		match.MatchDurationMS = durationMS.Int64
	}
	return match, err
}

func NewID() string {
	return uuid.NewString()
}

func nullableString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
