package domain

import "time"

type User struct {
	ID               string    `json:"id"`
	Username         string    `json:"username"`
	Email            string    `json:"email,omitempty"`
	AvatarURL        string    `json:"avatarUrl,omitempty"`
	ActiveSkin       string    `json:"activeSkin,omitempty"`
	CountryCode      string    `json:"countryCode,omitempty"`
	City             string    `json:"city,omitempty"`
	EloRating        int       `json:"eloRating"`
	PeakElo          int       `json:"peakElo"`
	Wins             int       `json:"wins"`
	Losses           int       `json:"losses"`
	Draws            int       `json:"draws"`
	CurrentStreak    int       `json:"currentStreak"`
	MaxStreak        int       `json:"maxStreak"`
	Experience       int       `json:"experience"`
	Level            int       `json:"level"`
	SubscriptionType string    `json:"subscriptionType"`
	CreatedAt        time.Time `json:"createdAt"`
}

type SessionTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresInSec int64  `json:"expiresInSec"`
}

type Puzzle struct {
	ID           string    `json:"id"`
	Difficulty   string    `json:"difficulty"`
	Seed         string    `json:"seed"`
	Solution     [9][9]int `json:"solution"`
	InitialBoard [9][9]int `json:"initialBoard"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Match struct {
	ID               string     `json:"id"`
	Player1ID        string     `json:"player1Id"`
	Player2ID        string     `json:"player2Id"`
	PuzzleID         string     `json:"puzzleId"`
	WinnerID         *string    `json:"winnerId,omitempty"`
	Player1EloBefore int        `json:"player1EloBefore"`
	Player1EloAfter  int        `json:"player1EloAfter"`
	Player2EloBefore int        `json:"player2EloBefore"`
	Player2EloAfter  int        `json:"player2EloAfter"`
	StartedAt        time.Time  `json:"startedAt"`
	EndedAt          *time.Time `json:"endedAt,omitempty"`
	MatchDurationMS  int64      `json:"matchDurationMs,omitempty"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type MatchMove struct {
	ID              string    `json:"id"`
	MatchID         string    `json:"matchId"`
	PlayerID        string    `json:"playerId"`
	RowIndex        int       `json:"rowIndex"`
	ColIndex        int       `json:"colIndex"`
	Value           int       `json:"value"`
	IsCorrect       bool      `json:"isCorrect"`
	MoveNumber      int       `json:"moveNumber"`
	TimeFromStartMS int64     `json:"timeFromStartMs"`
	CreatedAt       time.Time `json:"createdAt"`
}

type DailyChallenge struct {
	ID            string    `json:"id"`
	ChallengeDate string    `json:"challengeDate"`
	PuzzleID      string    `json:"puzzleId"`
	CreatedAt     time.Time `json:"createdAt"`
}

type DailyChallengeResult struct {
	ID               string    `json:"id"`
	UserID           string    `json:"userId"`
	ChallengeID      string    `json:"challengeId"`
	CompletionTimeMS int64     `json:"completionTimeMs"`
	MistakesCount    int       `json:"mistakesCount"`
	HintsUsed        int       `json:"hintsUsed"`
	Score            int       `json:"score"`
	Completed        bool      `json:"completed"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Skin struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	PreviewURL string    `json:"previewUrl,omitempty"`
	PriceUSD   float64   `json:"priceUsd"`
	IsPremium  bool      `json:"isPremium"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Analysis struct {
	MatchID   string         `json:"matchId"`
	Analysis  map[string]any `json:"analysis"`
	CreatedAt time.Time      `json:"createdAt"`
}
