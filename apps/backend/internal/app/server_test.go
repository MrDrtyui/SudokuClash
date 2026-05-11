package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sudoku-backend/internal/config"
	"sudoku-backend/internal/domain"
	"sudoku-backend/internal/platform/auth"
	"sudoku-backend/internal/platform/store"
	"sudoku-backend/internal/platform/ws"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type fakeStore struct {
	users         map[string]domain.User
	emailToID     map[string]string
	passwords     map[string]string
	sessions      map[string]fakeSession
	puzzle        domain.Puzzle
	matches       map[string]domain.Match
	matchHistory  []domain.Match
	moves         map[string][]domain.MatchMove
	analyses      map[string]domain.Analysis
	activeMatches map[string]string
	challenge     domain.DailyChallenge
	challengeData domain.Puzzle
	dailyResults  []domain.DailyChallengeResult
	skins         []domain.Skin
	ownedSkins    map[string][]domain.Skin
	subscription  map[string]any
	countries     []string
	cities        []string
	nextOpponent  string
	opponentQueue []string
}

type fakeSession struct {
	userID    string
	expiresAt time.Time
}

func newFakeStore(t *testing.T) *fakeStore {
	t.Helper()
	now := time.Now().UTC()
	puzzle := domain.Puzzle{
		ID:         "puzzle-1",
		Difficulty: "medium",
		Seed:       "seed-1",
		Solution: [9][9]int{
			{5, 3, 4, 6, 7, 8, 9, 1, 2},
			{6, 7, 2, 1, 9, 5, 3, 4, 8},
			{1, 9, 8, 3, 4, 2, 5, 6, 7},
			{8, 5, 9, 7, 6, 1, 4, 2, 3},
			{4, 2, 6, 8, 5, 3, 7, 9, 1},
			{7, 1, 3, 9, 2, 4, 8, 5, 6},
			{9, 6, 1, 5, 3, 7, 2, 8, 4},
			{2, 8, 7, 4, 1, 9, 6, 3, 5},
			{3, 4, 5, 2, 8, 6, 1, 7, 9},
		},
		InitialBoard: [9][9]int{
			{5, 3, 0, 0, 7, 0, 0, 0, 0},
			{6, 0, 0, 1, 9, 5, 0, 0, 0},
			{0, 9, 8, 0, 0, 0, 0, 6, 0},
			{8, 0, 0, 0, 6, 0, 0, 0, 3},
			{4, 0, 0, 8, 0, 3, 0, 0, 1},
			{7, 0, 0, 0, 2, 0, 0, 0, 6},
			{0, 6, 0, 0, 0, 0, 2, 8, 0},
			{0, 0, 0, 4, 1, 9, 0, 0, 5},
			{0, 0, 0, 0, 8, 0, 0, 7, 9},
		},
		CreatedAt: now,
	}

	opponent := domain.User{
		ID:               "user-opponent",
		Username:         "opponent",
		Email:            "opponent@example.com",
		CountryCode:      "KZ",
		City:             "Almaty",
		EloRating:        1150,
		PeakElo:          1150,
		SubscriptionType: "free",
		Level:            2,
		CreatedAt:        now,
	}
	opponentHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}

	match := domain.Match{
		ID:               "match-1",
		Player1ID:        opponent.ID,
		Player2ID:        "user-main",
		PuzzleID:         puzzle.ID,
		Player1EloBefore: 1150,
		Player1EloAfter:  1150,
		Player2EloBefore: 1000,
		Player2EloAfter:  1000,
		StartedAt:        now,
		Status:           "active",
		CreatedAt:        now,
	}

	return &fakeStore{
		users: map[string]domain.User{
			opponent.ID: opponent,
			"user-main": {
				ID:               "user-main",
				Username:         "mainuser",
				Email:            "main@example.com",
				CountryCode:      "KZ",
				City:             "Almaty",
				EloRating:        1000,
				PeakElo:          1000,
				SubscriptionType: "free",
				Level:            1,
				CreatedAt:        now,
			},
		},
		emailToID: map[string]string{
			"opponent@example.com": opponent.ID,
			"main@example.com":     "user-main",
		},
		passwords: map[string]string{
			"opponent@example.com": string(opponentHash),
		},
		sessions: map[string]fakeSession{},
		puzzle:   puzzle,
		matches: map[string]domain.Match{
			match.ID: match,
		},
		matchHistory: []domain.Match{match},
		moves: map[string][]domain.MatchMove{
			match.ID: {
				{
					ID:              "move-1",
					MatchID:         match.ID,
					PlayerID:        "user-main",
					RowIndex:        0,
					ColIndex:        2,
					Value:           4,
					IsCorrect:       true,
					MoveNumber:      1,
					TimeFromStartMS: 2000,
					CreatedAt:       now,
				},
			},
		},
		analyses:      map[string]domain.Analysis{},
		activeMatches: map[string]string{},
		challenge: domain.DailyChallenge{
			ID:            "daily-1",
			ChallengeDate: "2026-05-10",
			PuzzleID:      puzzle.ID,
			CreatedAt:     now,
		},
		challengeData: puzzle,
		dailyResults: []domain.DailyChallengeResult{
			{
				ID:               "daily-result-1",
				UserID:           "user-main",
				ChallengeID:      "daily-1",
				CompletionTimeMS: 120000,
				MistakesCount:    2,
				HintsUsed:        0,
				Score:            820,
				Completed:        true,
				CreatedAt:        now,
			},
		},
		skins: []domain.Skin{
			{ID: "skin-1", Name: "Classic", PriceUSD: 0, CreatedAt: now},
			{ID: "skin-2", Name: "Gold", PriceUSD: 4.99, IsPremium: true, CreatedAt: now},
		},
		ownedSkins: map[string][]domain.Skin{
			"user-main": {{ID: "skin-1", Name: "Classic", PriceUSD: 0, CreatedAt: now}},
		},
		subscription: map[string]any{"status": "free"},
		countries:    []string{"KZ"},
		cities:       []string{"Almaty"},
	}
}

func (f *fakeStore) CreateUser(_ context.Context, username, email, passwordHash string) (domain.User, error) {
	if _, exists := f.emailToID[email]; exists {
		return domain.User{}, errors.New("duplicate email")
	}
	id := "user-" + username
	user := domain.User{
		ID:               id,
		Username:         username,
		Email:            email,
		EloRating:        1000,
		PeakElo:          1000,
		Level:            1,
		SubscriptionType: "free",
		CreatedAt:        time.Now().UTC(),
	}
	f.users[id] = user
	f.emailToID[email] = id
	f.passwords[email] = passwordHash
	return user, nil
}

func (f *fakeStore) UserByEmail(_ context.Context, email string) (domain.User, string, error) {
	id, ok := f.emailToID[email]
	if !ok {
		return domain.User{}, "", errors.New("not found")
	}
	return f.users[id], f.passwords[email], nil
}

func (f *fakeStore) UserByID(_ context.Context, userID string) (domain.User, error) {
	user, ok := f.users[userID]
	if !ok {
		return domain.User{}, pgx.ErrNoRows
	}
	return user, nil
}

func (f *fakeStore) UpdateUserProfile(_ context.Context, userID, avatarURL, activeSkin, countryCode, city string) (domain.User, error) {
	user, ok := f.users[userID]
	if !ok {
		return domain.User{}, errors.New("not found")
	}
	if avatarURL != "" {
		user.AvatarURL = avatarURL
	}
	if activeSkin != "" {
		user.ActiveSkin = activeSkin
	}
	if countryCode != "" {
		user.CountryCode = countryCode
	}
	if city != "" {
		user.City = city
	}
	f.users[userID] = user
	return user, nil
}

func (f *fakeStore) CreateSession(_ context.Context, userID, refreshToken, _, _ string, expiresAt time.Time) error {
	f.sessions[refreshToken] = fakeSession{userID: userID, expiresAt: expiresAt}
	return nil
}

func (f *fakeStore) SessionByRefreshToken(_ context.Context, refreshToken string) (string, time.Time, error) {
	session, ok := f.sessions[refreshToken]
	if !ok {
		return "", time.Time{}, errors.New("not found")
	}
	return session.userID, session.expiresAt, nil
}

func (f *fakeStore) DeleteSession(_ context.Context, refreshToken string) error {
	delete(f.sessions, refreshToken)
	return nil
}

func (f *fakeStore) RandomPuzzle(context.Context, int) (domain.Puzzle, error) {
	return f.puzzle, nil
}

func (f *fakeStore) PuzzleByID(_ context.Context, puzzleID string) (domain.Puzzle, error) {
	if f.puzzle.ID != puzzleID {
		return domain.Puzzle{}, pgx.ErrNoRows
	}
	return f.puzzle, nil
}

func (f *fakeStore) CreateMatch(_ context.Context, player1ID, player2ID string, puzzle domain.Puzzle, player1Elo, player2Elo int) (domain.Match, error) {
	match := domain.Match{
		ID:               "match-created",
		Player1ID:        player1ID,
		Player2ID:        player2ID,
		PuzzleID:         puzzle.ID,
		Player1EloBefore: player1Elo,
		Player1EloAfter:  player1Elo,
		Player2EloBefore: player2Elo,
		Player2EloAfter:  player2Elo,
		StartedAt:        time.Now().UTC(),
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
	}
	f.matches[match.ID] = match
	f.matchHistory = append([]domain.Match{match}, f.matchHistory...)
	return match, nil
}

func (f *fakeStore) MatchByID(_ context.Context, matchID string) (domain.Match, error) {
	match, ok := f.matches[matchID]
	if !ok {
		return domain.Match{}, errors.New("not found")
	}
	return match, nil
}

func (f *fakeStore) MatchHistory(_ context.Context, _ string) ([]domain.Match, error) {
	return f.matchHistory, nil
}

func (f *fakeStore) RecordMove(_ context.Context, matchID, playerID string, move store.MatchSubmission, moveNumber int, elapsedMS int64, isCorrect bool) (domain.MatchMove, error) {
	saved := domain.MatchMove{
		ID:              "move-saved",
		MatchID:         matchID,
		PlayerID:        playerID,
		RowIndex:        move.Row,
		ColIndex:        move.Col,
		Value:           move.Value,
		IsCorrect:       isCorrect,
		MoveNumber:      moveNumber,
		TimeFromStartMS: elapsedMS,
		CreatedAt:       time.Now().UTC(),
	}
	f.moves[matchID] = append(f.moves[matchID], saved)
	return saved, nil
}

func (f *fakeStore) MatchMoves(_ context.Context, matchID string) ([]domain.MatchMove, error) {
	return f.moves[matchID], nil
}

func (f *fakeStore) FinishMatch(_ context.Context, match domain.Match, winnerID *string, durationMS int64) (domain.Match, error) {
	match.WinnerID = winnerID
	match.MatchDurationMS = durationMS
	match.Status = "finished"
	endedAt := time.Now().UTC()
	match.EndedAt = &endedAt
	f.matches[match.ID] = match
	return match, nil
}

func (f *fakeStore) SaveAnalysis(_ context.Context, matchID string, payload map[string]any) error {
	f.analyses[matchID] = domain.Analysis{MatchID: matchID, Analysis: payload, CreatedAt: time.Now().UTC()}
	return nil
}

func (f *fakeStore) AnalysisByMatchID(_ context.Context, matchID string) (domain.Analysis, error) {
	analysis, ok := f.analyses[matchID]
	if !ok {
		return domain.Analysis{}, errors.New("not found")
	}
	return analysis, nil
}

func (f *fakeStore) EnsureDailyChallenge(_ context.Context, _ string) (domain.DailyChallenge, domain.Puzzle, error) {
	return f.challenge, f.challengeData, nil
}

func (f *fakeStore) DailyResultByUserChallenge(_ context.Context, userID, challengeID string) (domain.DailyChallengeResult, error) {
	for _, result := range f.dailyResults {
		if result.UserID == userID && result.ChallengeID == challengeID {
			return result, nil
		}
	}
	return domain.DailyChallengeResult{}, pgx.ErrNoRows
}

func (f *fakeStore) UpsertDailyResult(_ context.Context, userID, challengeID string, completionTimeMS int64, mistakes, hints, score int, completed bool) (domain.DailyChallengeResult, error) {
	result := domain.DailyChallengeResult{
		ID:               "daily-result-created",
		UserID:           userID,
		ChallengeID:      challengeID,
		CompletionTimeMS: completionTimeMS,
		MistakesCount:    mistakes,
		HintsUsed:        hints,
		Score:            score,
		Completed:        completed,
		CreatedAt:        time.Now().UTC(),
	}
	f.dailyResults = append([]domain.DailyChallengeResult{result}, f.dailyResults...)
	return result, nil
}

func (f *fakeStore) DailyLeaderboard(_ context.Context, _ string) ([]domain.DailyChallengeResult, error) {
	return f.dailyResults, nil
}

func (f *fakeStore) GlobalLeaderboard(_ context.Context, _ int) ([]domain.User, error) {
	return []domain.User{f.users["user-main"], f.users["user-opponent"]}, nil
}

func (f *fakeStore) CountryLeaderboard(_ context.Context, _ string, _ int) ([]domain.User, error) {
	return []domain.User{f.users["user-main"]}, nil
}

func (f *fakeStore) CityLeaderboard(_ context.Context, _ string, _ int) ([]domain.User, error) {
	return []domain.User{f.users["user-main"]}, nil
}

func (f *fakeStore) DistinctCountries(context.Context) ([]string, error) { return f.countries, nil }
func (f *fakeStore) DistinctCities(context.Context) ([]string, error)    { return f.cities, nil }
func (f *fakeStore) ListSkins(context.Context) ([]domain.Skin, error)    { return f.skins, nil }

func (f *fakeStore) PurchaseSkin(_ context.Context, userID, skinID string) error {
	for _, skin := range f.skins {
		if skin.ID == skinID {
			f.ownedSkins[userID] = append(f.ownedSkins[userID], skin)
			return nil
		}
	}
	return errors.New("skin not found")
}

func (f *fakeStore) UserSkins(_ context.Context, userID string) ([]domain.Skin, error) {
	return f.ownedSkins[userID], nil
}

func (f *fakeStore) SubscriptionByUser(context.Context, string) (map[string]any, error) {
	return f.subscription, nil
}

func (f *fakeStore) CancelSubscription(_ context.Context, _ string) error {
	f.subscription = map[string]any{"status": "cancelled"}
	return nil
}

func (f *fakeStore) EnqueueMatchmaking(context.Context, string, string) error { return nil }
func (f *fakeStore) LeaveMatchmaking(context.Context, string, string) error   { return nil }

func (f *fakeStore) TryPopOpponent(_ context.Context, _, _ string) (string, error) {
	if len(f.opponentQueue) > 0 {
		opponent := f.opponentQueue[0]
		f.opponentQueue = f.opponentQueue[1:]
		return opponent, nil
	}
	opponent := f.nextOpponent
	f.nextOpponent = ""
	return opponent, nil
}

func (f *fakeStore) CacheMatchState(context.Context, string, map[string]any, time.Duration) error {
	return nil
}

func (f *fakeStore) MatchState(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (f *fakeStore) SetUserActiveMatch(_ context.Context, userID, matchID string, _ time.Duration) error {
	f.activeMatches[userID] = matchID
	return nil
}

func (f *fakeStore) UserActiveMatch(_ context.Context, userID string) (string, error) {
	return f.activeMatches[userID], nil
}

func (f *fakeStore) ClearUserActiveMatch(_ context.Context, userID string) error {
	delete(f.activeMatches, userID)
	return nil
}

func (f *fakeStore) SetUserOnline(context.Context, string, time.Duration) error { return nil }
func (f *fakeStore) RemoveUserOnline(context.Context, string) error             { return nil }

func newTestServer(t *testing.T) (*Server, *fakeStore) {
	t.Helper()
	fake := newFakeStore(t)
	srv := &Server{
		cfg: config.Config{
			HTTPAddr:        ":8080",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 24 * time.Hour,
		},
		store:    fake,
		auth:     auth.NewManager("test-secret"),
		hub:      ws.NewHub(),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		runtime:  &matchRuntime{matches: make(map[string]*liveMatch)},
	}
	return srv, fake
}

func TestRESTEndpointsAndDocs(t *testing.T) {
	srv, fake := newTestServer(t)
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	mainAccess := mustAccessToken(t, srv, "user-main")

	t.Run("healthz", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/healthz", "", nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"status":"ok"`)
	})

	t.Run("openapi", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/openapi.yaml", "", nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, "openapi: 3.0.3")
	})

	t.Run("swagger ui", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/docs", "", nil)
		assertStatus(t, resp, http.StatusOK)
		body := readBody(t, resp)
		if !strings.Contains(body, "SwaggerUIBundle") || !strings.Contains(body, "/openapi.yaml") {
			t.Fatalf("expected Swagger UI page to include bundle and spec url, got %s", body)
		}
	})

	var registerTokens domain.SessionTokens
	t.Run("auth register", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/auth/register", "", map[string]any{
			"username": "newuser",
			"email":    "new@example.com",
			"password": "password123",
		})
		assertStatus(t, resp, http.StatusCreated)
		var body struct {
			User   domain.User          `json:"user"`
			Tokens domain.SessionTokens `json:"tokens"`
		}
		decodeJSON(t, resp, &body)
		registerTokens = body.Tokens
		if body.User.Username != "newuser" {
			t.Fatalf("expected registered username, got %q", body.User.Username)
		}
	})

	t.Run("auth login", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/auth/login", "", map[string]any{
			"email":    "new@example.com",
			"password": "password123",
		})
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"accessToken"`)
	})

	t.Run("auth refresh", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/auth/refresh", "", map[string]any{
			"refreshToken": registerTokens.RefreshToken,
		})
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"accessToken"`)
	})

	t.Run("auth logout", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/auth/logout", mainAccess, map[string]any{
			"refreshToken": registerTokens.RefreshToken,
		})
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"ok":true`)
	})

	t.Run("users me", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/users/me", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"username":"mainuser"`)
	})

	t.Run("users me patch", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPatch, "/users/me", mainAccess, map[string]any{
			"avatarUrl":   "https://example.com/avatar.png",
			"countryCode": "US",
			"city":        "Boston",
		})
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"city":"Boston"`)
	})

	t.Run("users me skins", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/users/me/skins", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"name":"Classic"`)
	})

	t.Run("users public", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/users/user-opponent", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"username":"opponent"`)
	})

	t.Run("matchmaking join queued", func(t *testing.T) {
		fake.nextOpponent = ""
		resp := mustRequest(t, ts, http.MethodPost, "/matchmaking/join", mainAccess, map[string]any{"mode": "ranked"})
		assertStatus(t, resp, http.StatusAccepted)
		assertBodyContains(t, resp, `"status":"queued"`)
	})

	t.Run("matchmaking join matched", func(t *testing.T) {
		fake.nextOpponent = "user-opponent"
		resp := mustRequest(t, ts, http.MethodPost, "/matchmaking/join", mainAccess, map[string]any{"mode": "ranked"})
		assertStatus(t, resp, http.StatusCreated)
		body := readBody(t, resp)
		if !strings.Contains(body, `"status":"matched"`) || !strings.Contains(body, `"durationSeconds":120`) {
			t.Fatalf("expected matched payload with durationSeconds, got %s", body)
		}
	})

	t.Run("matchmaking skips stale queue entries", func(t *testing.T) {
		fake.opponentQueue = []string{"ghost-user", "user-opponent"}
		resp := mustRequest(t, ts, http.MethodPost, "/matchmaking/join", mainAccess, map[string]any{"mode": "ranked"})
		assertStatus(t, resp, http.StatusCreated)
		assertBodyContains(t, resp, `"opponentId":"user-opponent"`)
	})

	t.Run("matchmaking clears stale active runtime references", func(t *testing.T) {
		fake.activeMatches["user-main"] = "match-1"
		fake.nextOpponent = ""
		fake.opponentQueue = nil
		delete(srv.runtime.matches, "match-1")
		resp := mustRequest(t, ts, http.MethodPost, "/matchmaking/join", mainAccess, map[string]any{"mode": "ranked"})
		assertStatus(t, resp, http.StatusAccepted)
		assertBodyContains(t, resp, `"status":"queued"`)
	})

	t.Run("matchmaking leave", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/matchmaking/leave", mainAccess, map[string]any{"mode": "ranked"})
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"ok":true`)
	})

	t.Run("matches history", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/matches/history", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"id":"match-1"`)
	})

	t.Run("matches get by id", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/matches/match-1", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"puzzleId":"puzzle-1"`)
	})

	t.Run("matches replay", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/matches/match-1/replay", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"moveNumber":1`)
	})

	t.Run("matches analysis", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/matches/match-1/analysis", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"analysis"`)
	})

	t.Run("daily get", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/daily/", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"challengeDate":"2026-05-10"`)
	})

	t.Run("daily get includes my result when already completed", func(t *testing.T) {
		fake.dailyResults = append([]domain.DailyChallengeResult{{
			ID:               "daily-result-main",
			UserID:           "user-main",
			ChallengeID:      "daily-1",
			CompletionTimeMS: 310000,
			MistakesCount:    30,
			HintsUsed:        0,
			Score:            0,
			Completed:        true,
			CreatedAt:        time.Now().UTC(),
		}}, fake.dailyResults...)

		resp := mustRequest(t, ts, http.MethodGet, "/daily/", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		body := readBody(t, resp)
		if !strings.Contains(body, `"myResult"`) || !strings.Contains(body, `"completed":true`) || !strings.Contains(body, `"completionTimeMs":310000`) {
			t.Fatalf("expected completed myResult in body, got %s", body)
		}
	})

	t.Run("daily submit", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/daily/submit", mainAccess, map[string]any{
			"completionTimeMs": 90000,
			"mistakes":         1,
			"hintsUsed":        0,
		})
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"score":880`)
	})

	t.Run("daily leaderboard", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/daily/leaderboard", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"challengeId":"daily-1"`)
	})

	t.Run("leaderboards global", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/leaderboards/global", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"username":"mainuser"`)
	})

	t.Run("leaderboards countries", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/leaderboards/countries", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"KZ"`)
	})

	t.Run("leaderboards country", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/leaderboards/countries/KZ", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"username":"mainuser"`)
	})

	t.Run("leaderboards cities", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/leaderboards/cities", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"Almaty"`)
	})

	t.Run("leaderboards city", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/leaderboards/cities/Almaty", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"username":"mainuser"`)
	})

	t.Run("skins list", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/skins/", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"name":"Gold"`)
	})

	t.Run("skins purchase", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/skins/purchase", mainAccess, map[string]any{"skinId": "skin-1"})
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"ok":true`)
	})

	t.Run("skins purchase paid requires checkout", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/skins/purchase", mainAccess, map[string]any{"skinId": "skin-2"})
		assertStatus(t, resp, http.StatusPaymentRequired)
		assertBodyContains(t, resp, "paid skins require checkout")
	})

	t.Run("payments checkout session", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/payments/create-checkout-session", mainAccess, map[string]any{"skinId": "skin-2"})
		assertStatus(t, resp, http.StatusNotImplemented)
		assertBodyContains(t, resp, "stripe is not configured")
	})

	t.Run("payments webhook", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/payments/webhook", mainAccess, nil)
		assertStatus(t, resp, http.StatusNotImplemented)
		assertBodyContains(t, resp, "stripe webhook is not configured")
	})

	t.Run("subscription me", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/subscription/me", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"status":"free"`)
	})

	t.Run("subscription cancel", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodPost, "/subscription/cancel", mainAccess, nil)
		assertStatus(t, resp, http.StatusOK)
		assertBodyContains(t, resp, `"ok":true`)
	})

	t.Run("ws requires token", func(t *testing.T) {
		resp := mustRequest(t, ts, http.MethodGet, "/ws", "", nil)
		assertStatus(t, resp, http.StatusUnauthorized)
		assertBodyContains(t, resp, "missing token or matchId")
	})
}

func mustAccessToken(t *testing.T, srv *Server, userID string) string {
	t.Helper()
	token, _, err := srv.auth.IssueToken(userID, "access", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func mustRequest(t *testing.T, ts *httptest.Server, method, path, bearer string, body any) *http.Response {
	t.Helper()
	var payload *bytes.Reader
	if body == nil {
		payload = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, ts.URL+path, payload)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		var body bytes.Buffer
		_, _ = body.ReadFrom(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("expected status %d, got %d: %s", want, resp.StatusCode, body.String())
	}
}

func assertBodyContains(t *testing.T, resp *http.Response, part string) {
	t.Helper()
	body := readBody(t, resp)
	if !strings.Contains(body, part) {
		t.Fatalf("expected body to contain %q, got %s", part, body)
	}
}

func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatal(err)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var body bytes.Buffer
	_, _ = body.ReadFrom(resp.Body)
	return body.String()
}
