package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"sync"
	"time"

	"sudoku-backend/internal/config"
	"sudoku-backend/internal/docs"
	"sudoku-backend/internal/domain"
	"sudoku-backend/internal/platform/auth"
	"sudoku-backend/internal/platform/httpx"
	"sudoku-backend/internal/platform/storage"
	"sudoku-backend/internal/platform/store"
	"sudoku-backend/internal/platform/ws"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stripe/stripe-go/v82"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	cfg      config.Config
	http     *http.Server
	store    dataStore
	auth     *auth.Manager
	hub      *ws.Hub
	upgrader websocket.Upgrader
	runtime  *matchRuntime
	db       interface{ Close() }
	redis    interface{ Close() error }
}

type matchRuntime struct {
	mu      sync.RWMutex
	matches map[string]*liveMatch
}

type liveMatch struct {
	Match          domain.Match
	Puzzle         domain.Puzzle
	Boards         map[string][9][9]int
	Progress       map[string]int
	ScoredCells    map[string][9][9]bool
	TargetProgress int
	Moves          int
	StartedAt      time.Time
	Finished       bool
}

const matchDuration = 2 * time.Minute

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
var errLiveMatchNotFound = errors.New("live match not found")

type dataStore interface {
	CreateUser(ctx context.Context, username, email, passwordHash string) (domain.User, error)
	UserByEmail(ctx context.Context, email string) (domain.User, string, error)
	UserByID(ctx context.Context, userID string) (domain.User, error)
	UpdateUserProfile(ctx context.Context, userID, avatarURL, activeSkin, countryCode, city string) (domain.User, error)
	CreateSession(ctx context.Context, userID, refreshToken, ipAddress, userAgent string, expiresAt time.Time) error
	SessionByRefreshToken(ctx context.Context, refreshToken string) (string, time.Time, error)
	DeleteSession(ctx context.Context, refreshToken string) error
	RandomPuzzle(ctx context.Context, averageElo int) (domain.Puzzle, error)
	PuzzleByID(ctx context.Context, puzzleID string) (domain.Puzzle, error)
	CreateMatch(ctx context.Context, player1ID, player2ID string, puzzle domain.Puzzle, player1Elo, player2Elo int) (domain.Match, error)
	MatchByID(ctx context.Context, matchID string) (domain.Match, error)
	MatchHistory(ctx context.Context, userID string) ([]domain.Match, error)
	RecordMove(ctx context.Context, matchID, playerID string, move store.MatchSubmission, moveNumber int, elapsedMS int64, isCorrect bool) (domain.MatchMove, error)
	MatchMoves(ctx context.Context, matchID string) ([]domain.MatchMove, error)
	FinishMatch(ctx context.Context, match domain.Match, winnerID *string, durationMS int64) (domain.Match, error)
	SaveAnalysis(ctx context.Context, matchID string, payload map[string]any) error
	AnalysisByMatchID(ctx context.Context, matchID string) (domain.Analysis, error)
	EnsureDailyChallenge(ctx context.Context, date string) (domain.DailyChallenge, domain.Puzzle, error)
	DailyResultByUserChallenge(ctx context.Context, userID, challengeID string) (domain.DailyChallengeResult, error)
	UpsertDailyResult(ctx context.Context, userID, challengeID string, completionTimeMS int64, mistakes, hints, score int, completed bool) (domain.DailyChallengeResult, error)
	DailyLeaderboard(ctx context.Context, challengeID string) ([]domain.DailyChallengeResult, error)
	GlobalLeaderboard(ctx context.Context, limit int) ([]domain.User, error)
	CountryLeaderboard(ctx context.Context, country string, limit int) ([]domain.User, error)
	CityLeaderboard(ctx context.Context, city string, limit int) ([]domain.User, error)
	DistinctCountries(ctx context.Context) ([]string, error)
	DistinctCities(ctx context.Context) ([]string, error)
	ListSkins(ctx context.Context) ([]domain.Skin, error)
	PurchaseSkin(ctx context.Context, userID, skinID string) error
	UserSkins(ctx context.Context, userID string) ([]domain.Skin, error)
	SubscriptionByUser(ctx context.Context, userID string) (map[string]any, error)
	CancelSubscription(ctx context.Context, userID string) error
	EnqueueMatchmaking(ctx context.Context, userID, mode string) error
	LeaveMatchmaking(ctx context.Context, userID, mode string) error
	TryPopOpponent(ctx context.Context, userID, mode string) (string, error)
	CacheMatchState(ctx context.Context, matchID string, state map[string]any, ttl time.Duration) error
	MatchState(ctx context.Context, matchID string) (map[string]any, error)
	SetUserActiveMatch(ctx context.Context, userID, matchID string, ttl time.Duration) error
	UserActiveMatch(ctx context.Context, userID string) (string, error)
	ClearUserActiveMatch(ctx context.Context, userID string) error
	SetUserOnline(ctx context.Context, userID string, ttl time.Duration) error
	RemoveUserOnline(ctx context.Context, userID string) error
}

func NewServer(cfg config.Config) (*Server, error) {
	ctx := context.Background()
	db, err := storage.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		db.Close()
		return nil, err
	}

	appStore := store.New(db, redisClient)
	server := &Server{
		cfg:   cfg,
		store: appStore,
		auth:  auth.NewManager(cfg.JWTSecret),
		hub:   ws.NewHub(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		runtime: &matchRuntime{matches: make(map[string]*liveMatch)},
		db:      db,
		redis:   redisClient,
	}

	server.http = &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: server.router(),
	}
	return server, nil
}

func (s *Server) router() http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
	}))

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(docs.OpenAPI)
	})
	router.Get("/docs", s.handleSwaggerUI)
	router.Get("/docs/", s.handleSwaggerUI)
	router.Get("/ws", s.handleWS)
	router.Post("/payments/webhook", s.handlePaymentWebhook)

	apiRouter := chi.NewRouter()
	apiRouter.Use(middleware.Timeout(30 * time.Second))
	apiRouter.Mount("/", s.routes())

	router.Mount("/", apiRouter)
	return router
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", s.handleRegister)
		r.Post("/login", s.handleLogin)
		r.Post("/refresh", s.handleRefresh)
		r.With(s.authMiddleware).Post("/logout", s.handleLogout)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Route("/users", func(r chi.Router) {
			r.Get("/me", s.handleGetMe)
			r.Patch("/me", s.handlePatchMe)
			r.Get("/me/skins", s.handleMySkins)
			r.Get("/{id}", s.handleGetUser)
		})

		r.Route("/matchmaking", func(r chi.Router) {
			r.Post("/join", s.handleJoinMatchmaking)
			r.Post("/leave", s.handleLeaveMatchmaking)
		})

		r.Route("/matches", func(r chi.Router) {
			r.Get("/history", s.handleMatchHistory)
			r.Get("/{id}", s.handleGetMatch)
			r.Get("/{id}/replay", s.handleMatchReplay)
			r.Get("/{id}/analysis", s.handleMatchAnalysis)
		})

		r.Route("/daily", func(r chi.Router) {
			r.Get("/", s.handleDailyGet)
			r.Post("/submit", s.handleDailySubmit)
			r.Get("/leaderboard", s.handleDailyLeaderboard)
		})

		r.Route("/leaderboards", func(r chi.Router) {
			r.Get("/global", s.handleGlobalLeaderboard)
			r.Get("/countries", s.handleCountries)
			r.Get("/countries/{country}", s.handleCountryLeaderboard)
			r.Get("/cities", s.handleCities)
			r.Get("/cities/{city}", s.handleCityLeaderboard)
		})

		r.Route("/skins", func(r chi.Router) {
			r.Get("/", s.handleSkins)
			r.Post("/purchase", s.handlePurchaseSkin)
		})

		r.Route("/payments", func(r chi.Router) {
			r.Post("/create-checkout-session", s.handleCreateCheckoutSession)
			r.Get("/checkout-session/{id}", s.handleCheckoutSessionStatus)
		})

		r.Route("/subscription", func(r chi.Router) {
			r.Get("/me", s.handleSubscriptionMe)
			r.Post("/cancel", s.handleSubscriptionCancel)
		})
	})
	return r
}

func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func (s *Server) Close() {
	if s.db != nil {
		s.db.Close()
	}
	if s.redis != nil {
		_ = s.redis.Close()
	}
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			httpx.Error(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := s.auth.Parse(token)
		if err != nil || claims.Type != "access" {
			httpx.Error(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next.ServeHTTP(w, r.WithContext(httpx.WithUserID(r.Context(), claims.UserID)))
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if err := validateRegister(req.Username, req.Email, req.Password); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	user, err := s.store.CreateUser(r.Context(), req.Username, req.Email, string(hash))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	tokens, err := s.issueSession(r, user.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"user": user, "tokens": tokens})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if err := validateLogin(req.Email, req.Password); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	user, passwordHash, err := s.store.UserByEmail(r.Context(), req.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		httpx.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	tokens, err := s.issueSession(r, user.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": user, "tokens": tokens})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	claims, err := s.auth.Parse(req.RefreshToken)
	if err != nil || claims.Type != "refresh" {
		httpx.Error(w, http.StatusUnauthorized, "invalid token")
		return
	}
	userID, expiresAt, err := s.store.SessionByRefreshToken(r.Context(), req.RefreshToken)
	if err != nil || time.Now().After(expiresAt) {
		httpx.Error(w, http.StatusUnauthorized, "refresh session expired")
		return
	}
	_ = s.store.DeleteSession(r.Context(), req.RefreshToken)
	tokens, err := s.issueSession(r, userID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := s.store.DeleteSession(r.Context(), req.RefreshToken); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to delete session")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) issueSession(r *http.Request, userID string) (domain.SessionTokens, error) {
	access, _, err := s.auth.IssueToken(userID, "access", s.cfg.AccessTokenTTL)
	if err != nil {
		return domain.SessionTokens{}, err
	}
	refresh, expiresAt, err := s.auth.IssueToken(userID, "refresh", s.cfg.RefreshTokenTTL)
	if err != nil {
		return domain.SessionTokens{}, err
	}
	err = s.store.CreateSession(r.Context(), userID, refresh, r.RemoteAddr, r.UserAgent(), expiresAt)
	if err != nil {
		return domain.SessionTokens{}, err
	}
	return domain.SessionTokens{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresInSec: int64(s.cfg.AccessTokenTTL.Seconds()),
	}, nil
}

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.store.UserByID(r.Context(), httpx.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (s *Server) handlePatchMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AvatarURL   string `json:"avatarUrl"`
		ActiveSkin  string `json:"activeSkin"`
		CountryCode string `json:"countryCode"`
		City        string `json:"city"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	user, err := s.store.UpdateUserProfile(r.Context(), httpx.UserID(r.Context()), req.AvatarURL, req.ActiveSkin, req.CountryCode, req.City)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	user, err := s.store.UserByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	user.Email = ""
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (s *Server) handleJoinMatchmaking(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if req.Mode == "" {
		req.Mode = "ranked"
	}
	userID := httpx.UserID(r.Context())
	if err := s.store.SetUserOnline(r.Context(), userID, 2*time.Minute); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to update online state")
		return
	}
	if match, puzzle, opponentID, found, err := s.activeMatchPayload(r.Context(), userID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to resolve active match")
		return
	} else if found {
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{
			"status":          "matched",
			"matchId":         match.ID,
			"opponentId":      opponentID,
			"puzzle":          puzzle,
			"startedAt":       match.StartedAt,
			"durationSeconds": int(matchDuration.Seconds()),
		})
		return
	}
	match, puzzle, opponentID, err := s.tryCreateMatchFromQueue(r.Context(), userID, req.Mode)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to create match")
		return
	}
	if opponentID == "" {
		if err := s.store.EnqueueMatchmaking(r.Context(), userID, req.Mode); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to join queue")
			return
		}
		httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "mode": req.Mode})
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"status":          "matched",
		"matchId":         match.ID,
		"opponentId":      opponentID,
		"puzzle":          puzzle,
		"startedAt":       match.StartedAt,
		"durationSeconds": int(matchDuration.Seconds()),
	})
}

func (s *Server) activeMatchPayload(ctx context.Context, userID string) (domain.Match, domain.Puzzle, string, bool, error) {
	matchID, err := s.store.UserActiveMatch(ctx, userID)
	if err != nil || matchID == "" {
		return domain.Match{}, domain.Puzzle{}, "", false, nil
	}
	if _, _, runtimeErr := s.liveMatch(matchID); runtimeErr != nil {
		_ = s.store.ClearUserActiveMatch(ctx, userID)
		return domain.Match{}, domain.Puzzle{}, "", false, nil
	}
	match, err := s.store.MatchByID(ctx, matchID)
	if err != nil {
		_ = s.store.ClearUserActiveMatch(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Match{}, domain.Puzzle{}, "", false, nil
		}
		return domain.Match{}, domain.Puzzle{}, "", false, err
	}
	if match.Status != "active" {
		_ = s.store.ClearUserActiveMatch(ctx, userID)
		return domain.Match{}, domain.Puzzle{}, "", false, nil
	}
	puzzle, err := s.store.PuzzleByID(ctx, match.PuzzleID)
	if err != nil {
		return domain.Match{}, domain.Puzzle{}, "", false, err
	}
	opponentID := match.Player1ID
	if userID == match.Player1ID {
		opponentID = match.Player2ID
	}
	return match, puzzle, opponentID, true, nil
}

func (s *Server) tryCreateMatchFromQueue(ctx context.Context, userID, mode string) (domain.Match, domain.Puzzle, string, error) {
	const maxCandidates = 8

	for range maxCandidates {
		opponentID, err := s.store.TryPopOpponent(ctx, userID, mode)
		if err != nil {
			return domain.Match{}, domain.Puzzle{}, "", err
		}
		if opponentID == "" {
			return domain.Match{}, domain.Puzzle{}, "", nil
		}

		match, puzzle, err := s.createLiveMatch(ctx, opponentID, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return domain.Match{}, domain.Puzzle{}, "", err
		}
		return match, puzzle, opponentID, nil
	}

	return domain.Match{}, domain.Puzzle{}, "", nil
}

func (s *Server) handleLeaveMatchmaking(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	_ = httpx.ReadJSON(r, &req)
	if req.Mode == "" {
		req.Mode = "ranked"
	}
	if err := s.store.LeaveMatchmaking(r.Context(), httpx.UserID(r.Context()), req.Mode); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to leave queue")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) createLiveMatch(ctx context.Context, player1ID, player2ID string) (domain.Match, domain.Puzzle, error) {
	player1, err := s.store.UserByID(ctx, player1ID)
	if err != nil {
		return domain.Match{}, domain.Puzzle{}, fmt.Errorf("load player1: %w", err)
	}
	player2, err := s.store.UserByID(ctx, player2ID)
	if err != nil {
		return domain.Match{}, domain.Puzzle{}, fmt.Errorf("load player2: %w", err)
	}
	averageElo := (player1.EloRating + player2.EloRating) / 2
	puzzle, err := s.store.RandomPuzzle(ctx, averageElo)
	if err != nil {
		return domain.Match{}, domain.Puzzle{}, fmt.Errorf("load puzzle: %w", err)
	}
	match, err := s.store.CreateMatch(ctx, player1ID, player2ID, puzzle, player1.EloRating, player2.EloRating)
	if err != nil {
		return domain.Match{}, domain.Puzzle{}, fmt.Errorf("create match row: %w", err)
	}

	s.runtime.mu.Lock()
	s.runtime.matches[match.ID] = &liveMatch{
		Match:          match,
		Puzzle:         puzzle,
		Boards:         map[string][9][9]int{player1ID: puzzle.InitialBoard, player2ID: puzzle.InitialBoard},
		Progress:       map[string]int{player1ID: 0, player2ID: 0},
		ScoredCells:    map[string][9][9]bool{player1ID: {}, player2ID: {}},
		TargetProgress: countEmptyCells(puzzle.InitialBoard),
		StartedAt:      time.Now(),
	}
	s.runtime.mu.Unlock()

	state := map[string]any{
		"matchId":   match.ID,
		"player1Id": player1ID,
		"player2Id": player2ID,
		"status":    "active",
	}
	_ = s.store.CacheMatchState(ctx, match.ID, state, 2*time.Hour)
	_ = s.store.SetUserActiveMatch(ctx, player1ID, match.ID, 2*time.Hour)
	_ = s.store.SetUserActiveMatch(ctx, player2ID, match.ID, 2*time.Hour)

	s.hub.Broadcast(match.ID, "matchmaking:found", map[string]any{
		"matchId":   match.ID,
		"player1Id": player1ID,
		"player2Id": player2ID,
	})
	go s.runMatchTimer(match.ID)
	return match, puzzle, nil
}

func (s *Server) runMatchTimer(matchID string) {
	timer := time.NewTimer(matchDuration)
	defer timer.Stop()
	<-timer.C

	ctx := context.Background()
	match, live, err := s.liveMatch(matchID)
	if err != nil {
		return
	}

	var winnerID *string
	player1Progress := live.Progress[match.Player1ID]
	player2Progress := live.Progress[match.Player2ID]

	switch {
	case player1Progress > player2Progress:
		id := match.Player1ID
		winnerID = &id
	case player2Progress > player1Progress:
		id := match.Player2ID
		winnerID = &id
	}

	s.finishLiveMatch(ctx, matchID, winnerID)
}

func (s *Server) handleGetMatch(w http.ResponseWriter, r *http.Request) {
	match, err := s.store.MatchByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "match not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, match)
}

func (s *Server) handleMatchHistory(w http.ResponseWriter, r *http.Request) {
	matches, err := s.store.MatchHistory(r.Context(), httpx.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load history")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, matches)
}

func (s *Server) handleMatchReplay(w http.ResponseWriter, r *http.Request) {
	moves, err := s.store.MatchMoves(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load replay")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, moves)
}

func (s *Server) handleMatchAnalysis(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "id")
	analysis, err := s.store.AnalysisByMatchID(r.Context(), matchID)
	if err == nil {
		httpx.WriteJSON(w, http.StatusOK, analysis)
		return
	}
	match, matchErr := s.store.MatchByID(r.Context(), matchID)
	if matchErr != nil {
		httpx.Error(w, http.StatusNotFound, "analysis unavailable")
		return
	}
	puzzle, puzzleErr := s.store.PuzzleByID(r.Context(), match.PuzzleID)
	if puzzleErr != nil {
		httpx.Error(w, http.StatusNotFound, "analysis unavailable")
		return
	}
	moves, movesErr := s.store.MatchMoves(r.Context(), matchID)
	if movesErr != nil {
		httpx.Error(w, http.StatusNotFound, "analysis unavailable")
		return
	}
	payload := generateMatchAnalysis(match, puzzle, moves)
	if saveErr := s.store.SaveAnalysis(r.Context(), matchID, payload); saveErr != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to save analysis")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"matchId": matchID, "analysis": payload})
}

func generateMatchAnalysis(match domain.Match, puzzle domain.Puzzle, moves []domain.MatchMove) map[string]any {
	if len(moves) == 0 {
		return map[string]any{
			"summary": map[string]any{
				"winnerId":          match.WinnerID,
				"duration_ms":       match.MatchDurationMS,
				"difficulty":        puzzle.Difficulty,
				"seed":              puzzle.Seed,
				"total_moves":       0,
				"total_empty_cells": countEmptyCells(puzzle.InitialBoard),
			},
			"players": map[string]any{
				match.Player1ID: buildPlayerAnalysis(match.Player1ID, match, puzzle, moves),
				match.Player2ID: buildPlayerAnalysis(match.Player2ID, match, puzzle, moves),
			},
		}
	}

	player1Analysis := buildPlayerAnalysis(match.Player1ID, match, puzzle, moves)
	player2Analysis := buildPlayerAnalysis(match.Player2ID, match, puzzle, moves)
	globalInsights := buildGlobalInsights(match, player1Analysis, player2Analysis)

	return map[string]any{
		"summary": map[string]any{
			"winnerId":          match.WinnerID,
			"duration_ms":       match.MatchDurationMS,
			"difficulty":        puzzle.Difficulty,
			"seed":              puzzle.Seed,
			"total_moves":       len(moves),
			"player1_progress":  player1Analysis["progress"],
			"player2_progress":  player2Analysis["progress"],
			"total_empty_cells": countEmptyCells(puzzle.InitialBoard),
		},
		"players": map[string]any{
			match.Player1ID: player1Analysis,
			match.Player2ID: player2Analysis,
		},
		"insights": globalInsights,
	}
}

func buildPlayerAnalysis(playerID string, match domain.Match, puzzle domain.Puzzle, moves []domain.MatchMove) map[string]any {
	playerMoves := make([]domain.MatchMove, 0, len(moves))
	cellHits := map[string]int{}
	scoredCells := map[string]bool{}
	var correctMoves, incorrectMoves, hesitationCount, rushedMistakes, pressureMistakes, recoveryWindows int
	var totalDelta, correctDelta, incorrectDelta int64
	var previousPlayerMoveAt int64
	var streak, longestStreak int
	var lastMistakeIndex = -10
	mistakes := make([]map[string]any, 0)

	for _, move := range moves {
		if move.PlayerID != playerID {
			continue
		}
		playerMoves = append(playerMoves, move)
		cellKey := fmt.Sprintf("%d:%d", move.RowIndex, move.ColIndex)
		cellHits[cellKey]++

		delta := move.TimeFromStartMS
		if previousPlayerMoveAt > 0 {
			delta = move.TimeFromStartMS - previousPlayerMoveAt
		}
		if delta < 0 {
			delta = 0
		}
		previousPlayerMoveAt = move.TimeFromStartMS

		totalDelta += delta
		if delta >= 7000 {
			hesitationCount++
		}

		if move.IsCorrect {
			correctMoves++
			correctDelta += delta
			streak++
			if streak > longestStreak {
				longestStreak = streak
			}
			if puzzle.Solution[move.RowIndex][move.ColIndex] == move.Value {
				scoredCells[cellKey] = true
			}
			if len(playerMoves)-lastMistakeIndex <= 2 {
				recoveryWindows++
			}
		} else {
			incorrectMoves++
			incorrectDelta += delta
			streak = 0
			lastMistakeIndex = len(playerMoves)
			pressure := isPressureMoment(move.TimeFromStartMS, match.MatchDurationMS)
			if pressure {
				pressureMistakes++
			}
			if delta <= 1800 {
				rushedMistakes++
			}
			mistakes = append(mistakes, map[string]any{
				"time":               move.TimeFromStartMS,
				"row":                move.RowIndex,
				"col":                move.ColIndex,
				"value":              move.Value,
				"phase":              phaseForTime(move.TimeFromStartMS, match.MatchDurationMS),
				"think_time_ms":      delta,
				"pressure":           pressure,
				"message":            fmt.Sprintf("Incorrect placement at row %d, col %d", move.RowIndex+1, move.ColIndex+1),
				"recommendation":     recommendationForMistake(move, delta, pressure, cellHits[cellKey] > 1),
				"cell_revisited":     cellHits[cellKey] > 1,
				"expectedDigitKnown": puzzle.Solution[move.RowIndex][move.ColIndex] != 0,
			})
		}
	}

	totalMoves := len(playerMoves)
	progress := len(scoredCells)
	repeatedCells := 0
	for _, hits := range cellHits {
		if hits > 1 {
			repeatedCells++
		}
	}

	accuracy := 0
	if totalMoves > 0 {
		accuracy = int(math.Round(float64(correctMoves) / float64(totalMoves) * 100))
	}

	avgMoveTime := int64(0)
	if totalMoves > 0 {
		avgMoveTime = totalDelta / int64(totalMoves)
	}
	avgCorrectTime := int64(0)
	if correctMoves > 0 {
		avgCorrectTime = correctDelta / int64(correctMoves)
	}
	avgMistakeTime := int64(0)
	if incorrectMoves > 0 {
		avgMistakeTime = incorrectDelta / int64(incorrectMoves)
	}

	strengths := make([]string, 0, 4)
	if accuracy >= 85 {
		strengths = append(strengths, "Strong accuracy across the full board.")
	}
	if avgCorrectTime > 0 && avgCorrectTime <= 3500 {
		strengths = append(strengths, "Fast conversion on correct reads.")
	}
	if longestStreak >= 5 {
		strengths = append(strengths, "Built a long clean streak without dropping control.")
	}
	if pressureMistakes == 0 && totalMoves > 0 {
		strengths = append(strengths, "Stayed stable even in the closing seconds.")
	}
	if len(strengths) == 0 {
		strengths = append(strengths, "You kept the board moving and created a full replay to learn from.")
	}

	improvements := make([]string, 0, 4)
	if rushedMistakes > 0 {
		improvements = append(improvements, "Slow down on commit speed. Several mistakes came from very short think windows.")
	}
	if hesitationCount >= 3 {
		improvements = append(improvements, "Break long stalls faster. When a line freezes, rotate to another box instead of forcing the same read.")
	}
	if repeatedCells >= 3 {
		improvements = append(improvements, "You revisited the same cells often. Mark a clearer candidate before recommitting to that square.")
	}
	if pressureMistakes > 0 {
		improvements = append(improvements, "Late-game pressure created avoidable errors. Protect the final 30 seconds with simpler confirmations.")
	}
	if len(improvements) == 0 {
		improvements = append(improvements, "Next step: turn more clean reads into longer streaks so the board closes faster.")
	}

	insights := []string{
		fmt.Sprintf("Accuracy landed at %d%% over %d recorded moves.", accuracy, totalMoves),
		fmt.Sprintf("Average think time was %d ms, with %d unique cells solved.", avgMoveTime, progress),
	}
	if incorrectMoves > 0 {
		insights = append(insights, fmt.Sprintf("%d mistakes were logged and each one is broken down below.", incorrectMoves))
	}
	if recoveryWindows > 0 {
		insights = append(insights, fmt.Sprintf("You bounced back quickly after mistakes %d times.", recoveryWindows))
	}

	return map[string]any{
		"summary": map[string]any{
			"accuracy":               accuracy,
			"total_moves":            totalMoves,
			"correct_moves":          correctMoves,
			"incorrect_moves":        incorrectMoves,
			"avg_move_time_ms":       avgMoveTime,
			"avg_correct_time_ms":    avgCorrectTime,
			"avg_mistake_time_ms":    avgMistakeTime,
			"longest_correct_streak": longestStreak,
			"hesitation_count":       hesitationCount,
			"repeated_cells":         repeatedCells,
			"pressure_mistakes":      pressureMistakes,
			"recovery_windows":       recoveryWindows,
			"progress":               progress,
		},
		"progress":     progress,
		"mistakes":     mistakes,
		"strengths":    strengths,
		"improvements": improvements,
		"insights":     insights,
	}
}

func buildGlobalInsights(match domain.Match, player1Analysis, player2Analysis map[string]any) []string {
	player1Summary, _ := player1Analysis["summary"].(map[string]any)
	player2Summary, _ := player2Analysis["summary"].(map[string]any)
	player1Accuracy, _ := player1Summary["accuracy"].(int)
	player2Accuracy, _ := player2Summary["accuracy"].(int)
	player1Pressure, _ := player1Summary["pressure_mistakes"].(int)
	player2Pressure, _ := player2Summary["pressure_mistakes"].(int)

	insights := []string{
		fmt.Sprintf("Player 1 accuracy: %d%%. Player 2 accuracy: %d%%.", player1Accuracy, player2Accuracy),
		fmt.Sprintf("Pressure mistakes finished %d to %d in the last stretch.", player1Pressure, player2Pressure),
	}
	if match.WinnerID == nil {
		insights = append(insights, "The match ended as a draw, so consistency mattered more than one explosive streak.")
	} else {
		insights = append(insights, "The winner separated on fewer errors and cleaner conversion, not just raw speed.")
	}
	return insights
}

func recommendationForMistake(move domain.MatchMove, thinkTimeMS int64, pressure, revisited bool) string {
	switch {
	case pressure:
		return "This came under timer pressure. In the last stretch, favor safe singles over speculative placements."
	case revisited:
		return "You returned to this square again. Re-scan the full row, column, and box before recommitting here."
	case thinkTimeMS <= 1800:
		return "This was rushed. Add one extra verification pass before locking fast digits."
	case thinkTimeMS >= 7000:
		return "You spent a long time here and still missed it. Rotate to a different box when a read stalls."
	case move.RowIndex%3 == move.ColIndex%3:
		return "Check box constraints first here. This square is a good candidate for a cleaner box-level read."
	default:
		return "Before placing, confirm the digit against both the row and the column to reduce avoidable misses."
	}
}

func isPressureMoment(timeFromStartMS, durationMS int64) bool {
	if durationMS <= 0 {
		durationMS = matchDuration.Milliseconds()
	}
	return timeFromStartMS >= durationMS-30000
}

func phaseForTime(timeFromStartMS, durationMS int64) string {
	if durationMS <= 0 {
		durationMS = matchDuration.Milliseconds()
	}
	switch {
	case timeFromStartMS < durationMS/3:
		return "opening"
	case timeFromStartMS < (durationMS/3)*2:
		return "midgame"
	default:
		return "endgame"
	}
}

func (s *Server) handleDailyGet(w http.ResponseWriter, r *http.Request) {
	date := time.Now().Format("2006-01-02")
	challenge, puzzle, err := s.store.EnsureDailyChallenge(r.Context(), date)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load daily challenge")
		return
	}
	response := map[string]any{"challenge": challenge, "puzzle": puzzle}
	result, err := s.store.DailyResultByUserChallenge(r.Context(), httpx.UserID(r.Context()), challenge.ID)
	if err == nil {
		response["myResult"] = result
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusInternalServerError, "failed to load daily result")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (s *Server) handleDailySubmit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CompletionTimeMS int64 `json:"completionTimeMs"`
		Mistakes         int   `json:"mistakes"`
		HintsUsed        int   `json:"hintsUsed"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	challenge, _, err := s.store.EnsureDailyChallenge(r.Context(), time.Now().Format("2006-01-02"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to resolve challenge")
		return
	}
	score := 1000 - int(req.CompletionTimeMS/1000) - req.Mistakes*30 - req.HintsUsed*40
	if score < 0 {
		score = 0
	}
	result, err := s.store.UpsertDailyResult(r.Context(), httpx.UserID(r.Context()), challenge.ID, req.CompletionTimeMS, req.Mistakes, req.HintsUsed, score, true)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to submit result")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (s *Server) handleDailyLeaderboard(w http.ResponseWriter, r *http.Request) {
	challenge, _, err := s.store.EnsureDailyChallenge(r.Context(), time.Now().Format("2006-01-02"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to resolve challenge")
		return
	}
	results, err := s.store.DailyLeaderboard(r.Context(), challenge.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load leaderboard")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, results)
}

func (s *Server) handleGlobalLeaderboard(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.GlobalLeaderboard(r.Context(), 100)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load leaderboard")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, users)
}

func (s *Server) handleCountries(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.DistinctCountries(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load countries")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, values)
}

func (s *Server) handleCountryLeaderboard(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.CountryLeaderboard(r.Context(), chi.URLParam(r, "country"), 100)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load country leaderboard")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, users)
}

func (s *Server) handleCities(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.DistinctCities(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load cities")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, values)
}

func (s *Server) handleCityLeaderboard(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.CityLeaderboard(r.Context(), chi.URLParam(r, "city"), 100)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load city leaderboard")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, users)
}

func (s *Server) handleSkins(w http.ResponseWriter, r *http.Request) {
	skins, err := s.store.ListSkins(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load skins")
		return
	}
	if skins == nil {
		skins = []domain.Skin{}
	}
	httpx.WriteJSON(w, http.StatusOK, skins)
}

func (s *Server) handlePurchaseSkin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SkinID string `json:"skinId"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	skins, err := s.store.ListSkins(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load skins")
		return
	}
	for _, skin := range skins {
		if skin.ID == req.SkinID && skin.PriceUSD > 0 {
			httpx.Error(w, http.StatusPaymentRequired, "paid skins require checkout")
			return
		}
	}
	if err := s.store.PurchaseSkin(r.Context(), httpx.UserID(r.Context()), req.SkinID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to purchase skin")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMySkins(w http.ResponseWriter, r *http.Request) {
	skins, err := s.store.UserSkins(r.Context(), httpx.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load user skins")
		return
	}
	if skins == nil {
		skins = []domain.Skin{}
	}
	httpx.WriteJSON(w, http.StatusOK, skins)
}

func (s *Server) handleCreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	if s.cfg.StripeSecretKey == "" {
		httpx.Error(w, http.StatusNotImplemented, "stripe is not configured")
		return
	}
	var req struct {
		SkinID string `json:"skinId"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}
	user, err := s.store.UserByID(r.Context(), httpx.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	skins, err := s.store.ListSkins(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load skins")
		return
	}
	var selected *domain.Skin
	for _, skin := range skins {
		if skin.ID == req.SkinID {
			skinCopy := skin
			selected = &skinCopy
			break
		}
	}
	if selected == nil {
		httpx.Error(w, http.StatusNotFound, "skin not found")
		return
	}
	if selected.PriceUSD <= 0 {
		httpx.Error(w, http.StatusBadRequest, "free skins do not require checkout")
		return
	}

	stripe.Key = s.cfg.StripeSecretKey
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(fmt.Sprintf("%s/me?checkout=success&session_id={CHECKOUT_SESSION_ID}", strings.TrimRight(s.cfg.FrontendURL, "/"))),
		CancelURL:         stripe.String(fmt.Sprintf("%s/me?checkout=cancelled", strings.TrimRight(s.cfg.FrontendURL, "/"))),
		ClientReferenceID: stripe.String(user.ID),
		Metadata: map[string]string{
			"user_id": user.ID,
			"skin_id": selected.ID,
		},
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("usd"),
					UnitAmount: stripe.Int64(int64(math.Round(selected.PriceUSD * 100))),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Skin: " + selected.Name),
					},
				},
			},
		},
	}
	if user.Email != "" {
		params.CustomerEmail = stripe.String(user.Email)
	}
	session, err := checkoutsession.New(params)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to create checkout session")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":  session.ID,
		"url": session.URL,
	})
}

func (s *Server) handlePaymentWebhook(w http.ResponseWriter, r *http.Request) {
	if s.cfg.StripeWebhookSecret == "" {
		httpx.Error(w, http.StatusNotImplemented, "stripe webhook is not configured")
		return
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "failed to read payload")
		return
	}
	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), s.cfg.StripeWebhookSecret)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid webhook signature")
		return
	}
	if event.Type == "checkout.session.completed" {
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid checkout session payload")
			return
		}
		userID := session.Metadata["user_id"]
		skinID := session.Metadata["skin_id"]
		if userID != "" && skinID != "" {
			if err := s.store.PurchaseSkin(r.Context(), userID, skinID); err != nil {
				httpx.Error(w, http.StatusInternalServerError, "failed to grant skin")
				return
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCheckoutSessionStatus(w http.ResponseWriter, r *http.Request) {
	if s.cfg.StripeSecretKey == "" {
		httpx.Error(w, http.StatusNotImplemented, "stripe is not configured")
		return
	}
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		httpx.Error(w, http.StatusBadRequest, "missing session id")
		return
	}

	stripe.Key = s.cfg.StripeSecretKey
	session, err := checkoutsession.Get(sessionID, nil)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "failed to load checkout session")
		return
	}

	currentUserID := httpx.UserID(r.Context())
	if session.Metadata["user_id"] != currentUserID {
		httpx.Error(w, http.StatusForbidden, "checkout session does not belong to current user")
		return
	}

	paid := session.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid
	if paid {
		skinID := session.Metadata["skin_id"]
		if skinID != "" {
			if err := s.store.PurchaseSkin(r.Context(), currentUserID, skinID); err != nil {
				httpx.Error(w, http.StatusInternalServerError, "failed to grant skin")
				return
			}
		}
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":            session.ID,
		"status":        session.Status,
		"paymentStatus": session.PaymentStatus,
		"paid":          paid,
		"skinId":        session.Metadata["skin_id"],
	})
}

func (s *Server) handleSubscriptionMe(w http.ResponseWriter, r *http.Request) {
	subscription, err := s.store.SubscriptionByUser(r.Context(), httpx.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, subscription)
}

func (s *Server) handleSubscriptionCancel(w http.ResponseWriter, r *http.Request) {
	if err := s.store.CancelSubscription(r.Context(), httpx.UserID(r.Context())); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to cancel subscription")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	const page = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Sudoku Backend API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    html, body { margin: 0; padding: 0; background: #faf7f2; }
    body { font-family: ui-sans-serif, system-ui, sans-serif; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: "{{ .SpecURL }}",
      dom_id: "#swagger-ui",
      deepLinking: true,
      docExpansion: "list",
      tryItOutEnabled: true,
      persistAuthorization: true,
      displayRequestDuration: true
    });
  </script>
</body>
</html>`

	tmpl := template.Must(template.New("swagger-ui").Parse(page))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, map[string]string{"SpecURL": "/openapi.yaml"})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	matchID := r.URL.Query().Get("matchId")
	if token == "" || matchID == "" {
		httpx.Error(w, http.StatusUnauthorized, "missing token or matchId")
		return
	}
	claims, err := s.auth.Parse(token)
	if err != nil || claims.Type != "access" {
		httpx.Error(w, http.StatusUnauthorized, "invalid token")
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &ws.Connection{
		UserID:  claims.UserID,
		MatchID: matchID,
		Socket:  conn,
		Send:    make(chan []byte, 16),
	}
	s.hub.Register(client)
	defer s.hub.Unregister(client)

	go func() {
		for msg := range client.Send {
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		}
	}()

	wsCtx := context.Background()

	for {
		var payload struct {
			Event string         `json:"event"`
			Data  map[string]any `json:"data"`
		}
		if err := conn.ReadJSON(&payload); err != nil {
			return
		}
		switch payload.Event {
		case "move:submit":
			s.handleWSMove(wsCtx, client, payload.Data)
		case "match:surrender":
			s.handleWSSurrender(wsCtx, client)
		}
	}
}

func (s *Server) handleWSMove(ctx context.Context, client *ws.Connection, data map[string]any) {
	match, live, err := s.liveMatch(client.MatchID)
	if err != nil {
		if errors.Is(err, errLiveMatchNotFound) {
			return
		}
		s.hub.Broadcast(client.MatchID, "match:error", map[string]string{"message": err.Error()})
		return
	}
	row, col, value := intFromMap(data, "row"), intFromMap(data, "col"), intFromMap(data, "value")
	if row < 0 || row > 8 || col < 0 || col > 8 || value < 1 || value > 9 {
		s.hub.Broadcast(client.MatchID, "move:result", map[string]any{"correct": false, "message": "invalid move"})
		return
	}

	board := live.Boards[client.UserID]
	board[row][col] = value
	live.Boards[client.UserID] = board
	correct := live.Puzzle.Solution[row][col] == value
	if correct && !live.ScoredCells[client.UserID][row][col] {
		scoredCells := live.ScoredCells[client.UserID]
		scoredCells[row][col] = true
		live.ScoredCells[client.UserID] = scoredCells
		live.Progress[client.UserID]++
	}
	live.Moves++
	elapsedMS := time.Since(live.StartedAt).Milliseconds()
	saved, err := s.store.RecordMove(ctx, match.ID, client.UserID, store.MatchSubmission{Row: row, Col: col, Value: value}, live.Moves, elapsedMS, correct)
	if err != nil {
		log.Printf("record move failed: match=%s user=%s row=%d col=%d value=%d err=%v", match.ID, client.UserID, row, col, value, err)
		s.hub.Broadcast(client.MatchID, "match:error", map[string]string{"message": "failed to save move: " + err.Error()})
		return
	}
	_ = saved
	s.hub.Broadcast(client.MatchID, "move:result", map[string]any{
		"correct":     correct,
		"newProgress": live.Progress[client.UserID],
	})
	s.hub.Broadcast(client.MatchID, "match:update", map[string]any{
		"player1Progress": live.Progress[match.Player1ID],
		"player2Progress": live.Progress[match.Player2ID],
	})

	if live.Progress[client.UserID] >= live.TargetProgress {
		s.finishLiveMatch(ctx, match.ID, &client.UserID)
	}
}

func (s *Server) handleWSSurrender(ctx context.Context, client *ws.Connection) {
	match, _, err := s.liveMatch(client.MatchID)
	if err != nil {
		return
	}
	winnerID := match.Player1ID
	if client.UserID == match.Player1ID {
		winnerID = match.Player2ID
	}
	s.finishLiveMatch(ctx, match.ID, &winnerID)
}

func (s *Server) liveMatch(matchID string) (domain.Match, *liveMatch, error) {
	s.runtime.mu.RLock()
	defer s.runtime.mu.RUnlock()
	live := s.runtime.matches[matchID]
	if live == nil {
		return domain.Match{}, nil, errLiveMatchNotFound
	}
	return live.Match, live, nil
}

func (s *Server) finishLiveMatch(ctx context.Context, matchID string, winnerID *string) {
	s.runtime.mu.Lock()
	live := s.runtime.matches[matchID]
	if live == nil || live.Finished {
		s.runtime.mu.Unlock()
		return
	}
	live.Finished = true
	s.runtime.mu.Unlock()

	finished, err := s.store.FinishMatch(ctx, live.Match, winnerID, time.Since(live.StartedAt).Milliseconds())
	if err != nil {
		s.hub.Broadcast(matchID, "match:error", map[string]string{"message": "failed to finish match"})
		return
	}
	if moves, movesErr := s.store.MatchMoves(ctx, matchID); movesErr == nil {
		payload := generateMatchAnalysis(finished, live.Puzzle, moves)
		if saveErr := s.store.SaveAnalysis(ctx, matchID, payload); saveErr != nil {
			log.Printf("save analysis failed: match=%s err=%v", matchID, saveErr)
		}
	}
	change := finished.Player1EloAfter - finished.Player1EloBefore
	if winnerID != nil && *winnerID == finished.Player2ID {
		change = finished.Player2EloAfter - finished.Player2EloBefore
	}
	s.hub.Broadcast(matchID, "match:finished", map[string]any{
		"winnerId":         winnerID,
		"eloChange":        change,
		"player1Id":        finished.Player1ID,
		"player2Id":        finished.Player2ID,
		"player1EloBefore": finished.Player1EloBefore,
		"player1EloAfter":  finished.Player1EloAfter,
		"player2EloBefore": finished.Player2EloBefore,
		"player2EloAfter":  finished.Player2EloAfter,
		"matchDurationMs":  finished.MatchDurationMS,
		"player1Progress":  live.Progress[finished.Player1ID],
		"player2Progress":  live.Progress[finished.Player2ID],
	})
	_ = s.store.ClearUserActiveMatch(ctx, finished.Player1ID)
	_ = s.store.ClearUserActiveMatch(ctx, finished.Player2ID)
	s.runtime.mu.Lock()
	delete(s.runtime.matches, matchID)
	s.runtime.mu.Unlock()
}

func intFromMap(data map[string]any, key string) int {
	value, ok := data[key]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func validateRegister(username, email, password string) error {
	switch {
	case username == "":
		return errors.New("username is required")
	case len(username) < 3 || len(username) > 32:
		return errors.New("username must be between 3 and 32 characters")
	case !usernamePattern.MatchString(username):
		return errors.New("username may contain only letters, numbers, and underscores")
	case email == "":
		return errors.New("email is required")
	case len(email) > 255:
		return errors.New("email is too long")
	case !isValidEmail(email):
		return errors.New("email is invalid")
	case len(password) < 8:
		return errors.New("password must be at least 8 characters")
	case len(password) > 72:
		return errors.New("password is too long")
	default:
		return nil
	}
}

func validateLogin(email, password string) error {
	switch {
	case email == "":
		return errors.New("email is required")
	case !isValidEmail(email):
		return errors.New("email is invalid")
	case password == "":
		return errors.New("password is required")
	default:
		return nil
	}
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func countEmptyCells(board [9][9]int) int {
	empty := 0
	for row := range board {
		for col := range board[row] {
			if board[row][col] == 0 {
				empty++
			}
		}
	}
	return empty
}

var _ jwt.Claims = (*auth.Claims)(nil)
