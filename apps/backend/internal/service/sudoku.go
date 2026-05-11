package service

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
)

var baseSolution = [9][9]int{
	{5, 3, 4, 6, 7, 8, 9, 1, 2},
	{6, 7, 2, 1, 9, 5, 3, 4, 8},
	{1, 9, 8, 3, 4, 2, 5, 6, 7},
	{8, 5, 9, 7, 6, 1, 4, 2, 3},
	{4, 2, 6, 8, 5, 3, 7, 9, 1},
	{7, 1, 3, 9, 2, 4, 8, 5, 6},
	{9, 6, 1, 5, 3, 7, 2, 8, 4},
	{2, 8, 7, 4, 1, 9, 6, 3, 5},
	{3, 4, 5, 2, 8, 6, 1, 7, 9},
}

var masks = map[string][9][9]int{
	"easy": {
		{1, 1, 1, 0, 1, 0, 1, 1, 0},
		{1, 0, 1, 1, 1, 1, 0, 1, 0},
		{0, 1, 1, 0, 1, 1, 1, 0, 1},
		{1, 1, 0, 1, 1, 0, 0, 1, 1},
		{0, 1, 1, 1, 1, 1, 1, 1, 0},
		{1, 0, 0, 0, 1, 1, 0, 1, 1},
		{1, 0, 1, 1, 1, 0, 1, 1, 0},
		{0, 1, 0, 1, 1, 1, 1, 0, 1},
		{0, 1, 1, 0, 1, 0, 1, 1, 1},
	},
	"medium": {
		{1, 1, 0, 0, 1, 0, 0, 1, 0},
		{1, 0, 0, 1, 1, 1, 0, 0, 1},
		{0, 1, 1, 0, 0, 0, 1, 1, 0},
		{1, 0, 0, 0, 1, 0, 0, 0, 1},
		{1, 0, 1, 1, 0, 1, 1, 0, 1},
		{1, 0, 0, 0, 1, 0, 0, 0, 1},
		{0, 1, 1, 0, 0, 0, 1, 1, 0},
		{1, 0, 0, 1, 1, 1, 0, 0, 1},
		{0, 1, 0, 0, 1, 0, 0, 1, 1},
	},
	"hard": {
		{1, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 1, 0, 1, 0, 0, 0},
		{0, 1, 0, 0, 0, 0, 0, 1, 0},
		{0, 0, 0, 0, 1, 0, 0, 0, 1},
		{0, 0, 1, 0, 0, 0, 1, 0, 0},
		{1, 0, 0, 0, 1, 0, 0, 0, 0},
		{0, 1, 0, 0, 0, 0, 0, 1, 0},
		{0, 0, 0, 1, 0, 1, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 1},
	},
}

type GeneratedPuzzle struct {
	Seed         string
	Difficulty   string
	Solution     [9][9]int
	InitialBoard [9][9]int
}

func GeneratePuzzle(seed, difficulty string) GeneratedPuzzle {
	if _, ok := masks[difficulty]; !ok {
		difficulty = "medium"
	}

	rng := rand.New(rand.NewSource(hashSeed(seed)))
	solution := baseSolution
	mask := masks[difficulty]

	if rng.Intn(2) == 0 {
		solution = transpose(solution)
		mask = transpose(mask)
	}

	solution = permuteDigits(solution, rng)
	solution = permuteBands(solution, rng)
	solution = permuteRowsWithinBands(solution, rng)
	solution = permuteStacks(solution, rng)
	solution = permuteColsWithinStacks(solution, rng)

	mask = permuteBands(mask, rng)
	mask = permuteRowsWithinBands(mask, rng)
	mask = permuteStacks(mask, rng)
	mask = permuteColsWithinStacks(mask, rng)

	var initial [9][9]int
	for row := 0; row < 9; row++ {
		for col := 0; col < 9; col++ {
			if mask[row][col] == 1 {
				initial[row][col] = solution[row][col]
			}
		}
	}

	return GeneratedPuzzle{
		Seed:         seed,
		Difficulty:   difficulty,
		Solution:     solution,
		InitialBoard: initial,
	}
}

func EncodeBoard(board [9][9]int) ([]byte, error) {
	return json.Marshal(board)
}

func RankedDifficultyForElo(seed string, averageElo int) string {
	switch {
	case averageElo < 1100:
		return "easy"
	case averageElo < 1400:
		if hashSeed(seed)%4 == 0 {
			return "medium"
		}
		return "easy"
	case averageElo < 1700:
		return "medium"
	case averageElo < 2000:
		if hashSeed(seed)%4 == 0 {
			return "hard"
		}
		return "medium"
	default:
		if hashSeed(seed)%3 == 0 {
			return "medium"
		}
		return "hard"
	}
}

func DailyDifficulty(seed string) string {
	switch hashSeed(seed) % 4 {
	case 0:
		return "easy"
	case 1, 2:
		return "medium"
	default:
		return "easy"
	}
}

func DailySeed(date string) string {
	return fmt.Sprintf("daily:%s", date)
}

func RankedSeed(entropy string) string {
	return fmt.Sprintf("ranked:%s", entropy)
}

func hashSeed(seed string) int64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(seed))
	return int64(hasher.Sum64())
}

func transpose[T int](board [9][9]T) [9][9]T {
	var out [9][9]T
	for row := 0; row < 9; row++ {
		for col := 0; col < 9; col++ {
			out[row][col] = board[col][row]
		}
	}
	return out
}

func permuteDigits(board [9][9]int, rng *rand.Rand) [9][9]int {
	digits := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	rng.Shuffle(len(digits), func(i, j int) {
		digits[i], digits[j] = digits[j], digits[i]
	})
	var mapping [10]int
	for idx, digit := range digits {
		mapping[idx+1] = digit
	}
	var out [9][9]int
	for row := 0; row < 9; row++ {
		for col := 0; col < 9; col++ {
			out[row][col] = mapping[board[row][col]]
		}
	}
	return out
}

func permuteBands[T int](board [9][9]T, rng *rand.Rand) [9][9]T {
	order := []int{0, 1, 2}
	rng.Shuffle(len(order), func(i, j int) {
		order[i], order[j] = order[j], order[i]
	})
	var out [9][9]T
	for newBand, oldBand := range order {
		for offset := 0; offset < 3; offset++ {
			out[newBand*3+offset] = board[oldBand*3+offset]
		}
	}
	return out
}

func permuteRowsWithinBands[T int](board [9][9]T, rng *rand.Rand) [9][9]T {
	out := board
	for band := 0; band < 3; band++ {
		order := []int{0, 1, 2}
		rng.Shuffle(len(order), func(i, j int) {
			order[i], order[j] = order[j], order[i]
		})
		var rows [3][9]T
		for idx, oldRow := range order {
			rows[idx] = out[band*3+oldRow]
		}
		for idx := 0; idx < 3; idx++ {
			out[band*3+idx] = rows[idx]
		}
	}
	return out
}

func permuteStacks[T int](board [9][9]T, rng *rand.Rand) [9][9]T {
	transposed := transpose(board)
	transposed = permuteBands(transposed, rng)
	return transpose(transposed)
}

func permuteColsWithinStacks[T int](board [9][9]T, rng *rand.Rand) [9][9]T {
	transposed := transpose(board)
	transposed = permuteRowsWithinBands(transposed, rng)
	return transpose(transposed)
}
