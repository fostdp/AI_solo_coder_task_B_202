package simulation

import (
	"math"
	"time"

	"bianzhong-acoustic-system/models"
)

type GradientDescentOptimizer struct {
	Bell              *models.Bell
	LearningRate      float64
	MaxIterations     int
	ToleranceHz       float64
	ConvergenceFactor float64
}

func NewGradientDescentOptimizer(bell *models.Bell) *GradientDescentOptimizer {
	return &GradientDescentOptimizer{
		Bell:              bell,
		LearningRate:      0.05,
		MaxIterations:     100,
		ToleranceHz:       0.05,
		ConvergenceFactor: 1e-6,
	}
}

func (g *GradientDescentOptimizer) frequencyToCents(freq, target float64) float64 {
	return 1200.0 * math.Log2(freq/target)
}

func (g *GradientDescentOptimizer) generateCandidatePositions() []models.GrindingPosition {
	positions := make([]models.GrindingPosition, 0)

	radiusCm := g.Bell.DiameterCm / 2.0
	heightCm := g.Bell.HeightCm

	for i := 0; i < 8; i++ {
		theta := 2.0 * math.Pi * float64(i) / 8.0
		for j := 1; j < 5; j++ {
			yRatio := float64(j) / 5.0
			r := radiusCm * (0.85 - 0.2*yRatio)
			pos := models.GrindingPosition{
				X: r * math.Cos(theta),
				Y: yRatio * heightCm,
				Z: r * math.Sin(theta),
			}
			positions = append(positions, pos)
		}
	}

	criticalZones := []models.GrindingPosition{
		{X: 0, Y: heightCm * 0.15, Z: radiusCm * 0.8},
		{X: 0, Y: heightCm * 0.5, Z: radiusCm * 0.75},
		{X: 0, Y: heightCm * 0.85, Z: radiusCm * 0.7},
		{X: radiusCm * 0.8 * math.Cos(math.Pi/4), Y: heightCm * 0.3, Z: radiusCm * 0.8 * math.Sin(math.Pi/4)},
		{X: radiusCm * 0.75 * math.Cos(3*math.Pi/4), Y: heightCm * 0.6, Z: radiusCm * 0.75 * math.Sin(3*math.Pi/4)},
	}
	positions = append(positions, criticalZones...)

	return positions
}

func (g *GradientDescentOptimizer) computeGradient(sim *FEMSimulator, pos models.GrindingPosition) float64 {
	return sim.CalculateFrequencySensitivity(pos)
}

func (g *GradientDescentOptimizer) OptimizePitch(
	currentFreq, targetFreq float64,
) *models.PitchCorrection {

	sim := NewFEMSimulator(g.Bell)
	sim.GenerateGrid()

	candidates := g.generateCandidatePositions()

	deviationCents := g.frequencyToCents(currentFreq, targetFreq)

	if math.Abs(deviationCents) <= g.Bell.ToleranceCents {
		return &models.PitchCorrection{
			CreatedAt:            time.Now(),
			BellID:               g.Bell.ID,
			CurrentFrequency:     currentFreq,
			TargetFrequency:      targetFreq,
			DeviationCents:       deviationCents,
			RecommendedPositions: []models.CorrectionRecommendation{},
			EstimatedResultFreq:  currentFreq,
			Iterations:           0,
			Algorithm:            "gradient_descent",
			Status:               "within_tolerance",
		}
	}

	type positionScore struct {
		pos         models.GrindingPosition
		gradient    float64
		sensitivity float64
	}

	scoredPositions := make([]positionScore, 0, len(candidates))

	for _, pos := range candidates {
		sensitivity := g.computeGradient(sim, pos)

		var score float64
		if deviationCents > 0 {
			score = -sensitivity
		} else {
			score = sensitivity
		}

		scoredPositions = append(scoredPositions, positionScore{
			pos:         pos,
			gradient:    score,
			sensitivity: sensitivity,
		})
	}

	for i := 0; i < len(scoredPositions)-1; i++ {
		for j := i + 1; j < len(scoredPositions); j++ {
			if scoredPositions[j].gradient > scoredPositions[i].gradient {
				scoredPositions[i], scoredPositions[j] = scoredPositions[j], scoredPositions[i]
			}
		}
	}

	recommendations := make([]models.CorrectionRecommendation, 0)
	remainingDeviationCents := deviationCents
	estimatedFreq := currentFreq
	totalDepthUsed := 0.0
	iterations := 0
	maxTotalDepth := g.Bell.MaxGrindingDepthMm

	numPositions := 3
	if len(scoredPositions) < numPositions {
		numPositions = len(scoredPositions)
	}

	for idx := 0; idx < numPositions; idx++ {
		if iterations >= g.MaxIterations {
			break
		}
		if math.Abs(remainingDeviationCents) <= g.Bell.ToleranceCents {
			break
		}
		if totalDepthUsed >= maxTotalDepth {
			break
		}

		sp := scoredPositions[idx]

		maxAllowedDepth := maxTotalDepth - totalDepthUsed
		if maxAllowedDepth <= 0 {
			break
		}

		var desiredDepth float64
		if math.Abs(sp.sensitivity) > 1e-6 {
			targetFreqChange := targetFreq - estimatedFreq
			desiredDepth = math.Abs(targetFreqChange / sp.sensitivity)

			desiredDepth *= g.LearningRate

			depthLimit := maxAllowedDepth / float64(numPositions-idx)
			if desiredDepth > depthLimit {
				desiredDepth = depthLimit
			}

			if desiredDepth < 0.01 {
				desiredDepth = 0.01
			}
		} else {
			desiredDepth = 0.05
		}

		depthSign := 1.0
		if sp.sensitivity < 0 && remainingDeviationCents > 0 {
			depthSign = 1.0
		} else if sp.sensitivity > 0 && remainingDeviationCents < 0 {
			depthSign = 1.0
		}

		actualDepth := desiredDepth * depthSign
		if actualDepth < 0 {
			actualDepth = math.Abs(actualDepth)
		}

		freqChange := sp.sensitivity * actualDepth

		if remainingDeviationCents > 0 && freqChange > 0 {
			freqChange = -freqChange
		} else if remainingDeviationCents < 0 && freqChange < 0 {
			freqChange = -freqChange
		}

		newEstimatedFreq := estimatedFreq + freqChange
		newDeviationCents := g.frequencyToCents(newEstimatedFreq, targetFreq)

		if math.Abs(newDeviationCents) < math.Abs(remainingDeviationCents) {
			recommendations = append(recommendations, models.CorrectionRecommendation{
				Position:          sp.pos,
				DepthMm:           actualDepth,
				Sensitivity:       sp.sensitivity,
				FrequencyChangeHz: freqChange,
			})

			estimatedFreq = newEstimatedFreq
			remainingDeviationCents = newDeviationCents
			totalDepthUsed += actualDepth
			iterations++

			sim.ApplyGrinding(sp.pos, actualDepth)
		}
	}

	status := "recommended"
	if math.Abs(remainingDeviationCents) <= g.Bell.ToleranceCents {
		status = "achievable"
	} else if totalDepthUsed >= maxTotalDepth {
		status = "depth_limit_reached"
	}

	return &models.PitchCorrection{
		CreatedAt:            time.Now(),
		BellID:               g.Bell.ID,
		CurrentFrequency:     currentFreq,
		TargetFrequency:      targetFreq,
		DeviationCents:       deviationCents,
		RecommendedPositions: recommendations,
		EstimatedResultFreq:  estimatedFreq,
		Iterations:           iterations,
		Algorithm:            "gradient_descent",
		Status:               status,
	}
}
