package utils

import (
	"math"

	"github.com/chess-vn/slchess/internal/domains/entities"
)

// Constants
var q = math.Log(10) / 400 // Glicko scaling constant

// g(RD) function
func g(rd float64) float64 {
	return 1 / math.Sqrt(1+3*q*q*rd*rd/(math.Pi*math.Pi))
}

// Expected score function
func expectedScore(r1, r2, rd2 float64) float64 {
	return 1 / (1 + math.Pow(10, -g(rd2)*(r1-r2)/400))
}

// Update rating and deviation
func UpdateRating(player entities.Player, opponents []entities.Player, results []float64) entities.Player {
	if len(opponents) != len(results) {
		panic("Mismatch between opponents and results")
	}

	var d2, sum float64
	for i, opp := range opponents {
		E := expectedScore(player.Rating, opp.Rating, opp.RD)
		gRD := g(opp.RD)
		d2 += (q * q * gRD * gRD * E * (1 - E))
		sum += gRD * (results[i] - E)
	}

	d2 = 1 / d2
	newRD := math.Sqrt(1 / (1/(player.RD*player.RD) + 1/d2))
	newRating := player.Rating + (q/(1/(player.RD*player.RD)+1/d2))*sum

	player.Rating = newRating
	player.RD = newRD
	return player
}
