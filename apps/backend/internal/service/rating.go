package service

import "math"

func CalculateElo(playerRating, opponentRating int, score float64) int {
	expected := 1.0 / (1.0 + math.Pow(10, float64(opponentRating-playerRating)/400.0))
	return playerRating + int(math.Round(32*(score-expected)))
}
