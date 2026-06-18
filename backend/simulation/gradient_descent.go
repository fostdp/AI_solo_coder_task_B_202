package simulation

import (
	"math"
	"time"

	"bianzhong-acoustic-system/models"
)

const (
	DefaultRho         = 100.0
	RhoIncreaseFactor  = 10.0
	MaxRho             = 1e8
	LambdaTolerance    = 1e-4
	ConstraintTolerance = 1e-4
	MaxOuterIterations  = 15
	BacktrackAlpha      = 0.5
	BacktrackBeta       = 0.5
	MaxBacktrackSteps   = 20
	MinDepthPerPosition = 0.005
	MaxDepthPerPosition = 0.8

	BoundaryDampingZone = 0.05
	BoundaryDampingFactor = 0.2
	OscillationLookback = 3
	OscillationThreshold = 0.6
)

type OptimizationVariable struct {
	Position      models.GrindingPosition
	Depth         float64
	Sensitivity   float64
	Active        bool
	LowerBound    float64
	UpperBound    float64
	DepthHistory  []float64
	GradHistory   []float64
}

type Constraint struct {
	Name        string
	Lambda      float64
	Value       float64
	Gradient    []float64
	IsSatisfied bool
	Type        string
}

type GradientDescentOptimizer struct {
	Bell              *models.Bell
	LearningRate      float64
	MaxIterations     int
	ToleranceHz       float64
	ConvergenceFactor float64
	Rho               float64
	OuterIterations   int
}

func NewGradientDescentOptimizer(bell *models.Bell) *GradientDescentOptimizer {
	return &GradientDescentOptimizer{
		Bell:              bell,
		LearningRate:      0.05,
		MaxIterations:     100,
		ToleranceHz:       0.05,
		ConvergenceFactor: 1e-6,
		Rho:               DefaultRho,
		OuterIterations:   MaxOuterIterations,
	}
}

func (g *GradientDescentOptimizer) frequencyToCents(freq, target float64) float64 {
	return 1200.0 * math.Log2(freq/target)
}

func (g *GradientDescentOptimizer) centsToFrequency(cents, target float64) float64 {
	return target * math.Pow(2, cents/1200.0)
}

func (g *GradientDescentOptimizer) detectOscillation(v *OptimizationVariable) bool {
	if len(v.DepthHistory) < OscillationLookback*2 {
		return false
	}
	signChanges := 0
	recent := v.DepthHistory[len(v.DepthHistory)-OscillationLookback*2:]
	for i := 1; i < len(recent); i++ {
		delta := recent[i] - recent[i-1]
		prevDelta := recent[i-1]
		if i > 1 {
			prevDelta = recent[i-1] - recent[i-2]
		}
		if delta*prevDelta < 0 && math.Abs(delta) > OscillationThreshold*v.UpperBound {
			signChanges++
		}
	}
	return signChanges >= OscillationLookback-1
}

func (g *GradientDescentOptimizer) boundaryDamping(v *OptimizationVariable, grad float64) float64 {
	rangeSize := v.UpperBound - v.LowerBound
	if rangeSize <= 0 {
		return grad
	}
	zone := BoundaryDampingZone * rangeSize
	damping := 1.0
	distToLower := v.Depth - v.LowerBound
	distToUpper := v.UpperBound - v.Depth

	if distToLower < zone {
		if grad < 0 {
			t := distToLower / zone
			damping = BoundaryDampingFactor + (1-BoundaryDampingFactor)*t*t
		}
	}
	if distToUpper < zone {
		if grad > 0 {
			t := distToUpper / zone
			damping = BoundaryDampingFactor + (1-BoundaryDampingFactor)*t*t
		}
	}
	return grad * damping
}

func (g *GradientDescentOptimizer) kktSatisfied(v *OptimizationVariable, grad float64) bool {
	rangeSize := v.UpperBound - v.LowerBound
	if rangeSize <= 0 {
		rangeSize = 1.0
	}
	tol := 1e-6 * rangeSize
	if v.Depth <= v.LowerBound+tol && grad >= -tol {
		return true
	}
	if v.Depth >= v.UpperBound-tol && grad <= tol {
		return true
	}
	if v.Depth > v.LowerBound+tol && v.Depth < v.UpperBound-tol && math.Abs(grad) <= tol {
		return true
	}
	return false
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

func (g *GradientDescentOptimizer) objectiveFunction(vars []*OptimizationVariable, currentFreq, targetFreq float64) (float64, []float64) {
	sim := NewFEMSimulator(g.Bell)
	sim.GenerateGrid()

	estimatedFreq := currentFreq
	gradients := make([]float64, len(vars))

	for i, v := range vars {
		if v.Depth <= 0 {
			gradients[i] = 0
			continue
		}
		sim.ApplyGrinding(v.Position, v.Depth)
		freqChange := v.Sensitivity * v.Depth

		if currentFreq > targetFreq && freqChange > 0 {
			freqChange = -freqChange
		} else if currentFreq < targetFreq && freqChange < 0 {
			freqChange = -freqChange
		}
		estimatedFreq += freqChange
	}

	deviation := estimatedFreq - targetFreq
	objective := 0.5 * deviation * deviation

	for i, v := range vars {
		if v.Sensitivity == 0 {
			gradients[i] = 0
			continue
		}

		effectiveSens := v.Sensitivity
		if currentFreq > targetFreq && effectiveSens > 0 {
			effectiveSens = -effectiveSens
		} else if currentFreq < targetFreq && effectiveSens < 0 {
			effectiveSens = -effectiveSens
		}

		gradients[i] = deviation * effectiveSens
	}

	return objective, gradients
}

func (g *GradientDescentOptimizer) computeConstraints(vars []*OptimizationVariable, totalMaxDepth float64) []*Constraint {
	constraints := make([]*Constraint, 0)

	totalDepth := 0.0
	for _, v := range vars {
		if v.Active {
			totalDepth += v.Depth
		}
	}

	totalGrad := make([]float64, len(vars))
	for i, v := range vars {
		if v.Active {
			totalGrad[i] = 1.0
		}
	}
	constraints = append(constraints, &Constraint{
		Name:        "total_depth",
		Value:       totalDepth - totalMaxDepth,
		Gradient:    totalGrad,
		IsSatisfied: totalDepth <= totalMaxDepth+ConstraintTolerance,
		Type:        "inequality",
	})

	for i, v := range vars {
		lowerGrad := make([]float64, len(vars))
		lowerGrad[i] = -1.0
		constraints = append(constraints, &Constraint{
			Name:        "lower_bound_" + string(rune('0'+i)),
			Value:       MinDepthPerPosition - v.Depth,
			Gradient:    lowerGrad,
			IsSatisfied: v.Depth >= MinDepthPerPosition-ConstraintTolerance,
			Type:        "inequality",
		})

		upperGrad := make([]float64, len(vars))
		upperGrad[i] = 1.0
		constraints = append(constraints, &Constraint{
			Name:        "upper_bound_" + string(rune('0'+i)),
			Value:       v.Depth - v.UpperBound,
			Gradient:    upperGrad,
			IsSatisfied: v.Depth <= v.UpperBound+ConstraintTolerance,
			Type:        "inequality",
		})
	}

	return constraints
}

func (g *GradientDescentOptimizer) augmentedLagrangian(
	vars []*OptimizationVariable,
	currentFreq, targetFreq float64,
	constraints []*Constraint,
	totalMaxDepth float64,
) (float64, []float64) {

	obj, objGrad := g.objectiveFunction(vars, currentFreq, targetFreq)
	constraints = g.computeConstraints(vars, totalMaxDepth)

	augmentedObj := obj
	augmentedGrad := make([]float64, len(objGrad))
	copy(augmentedGrad, objGrad)

	for _, c := range constraints {
		violation := math.Max(0, c.Value)
		augmentedObj += c.Lambda*violation + 0.5*g.Rho*violation*violation

		for i := range augmentedGrad {
			active := c.Value > -ConstraintTolerance
			if active {
				gradComponent := c.Lambda*c.Gradient[i] + g.Rho*math.Max(0, c.Value)*c.Gradient[i]
				augmentedGrad[i] += gradComponent
			}
		}
	}

	return augmentedObj, augmentedGrad
}

func (g *GradientDescentOptimizer) lineSearch(
	vars []*OptimizationVariable,
	currentFreq, targetFreq float64,
	direction []float64,
	currentObj float64,
	currentGrad []float64,
	constraints []*Constraint,
	totalMaxDepth float64,
) float64 {

	step := g.LearningRate

	dotGradDir := 0.0
	for i := range direction {
		dotGradDir += currentGrad[i] * direction[i]
	}

	for stepIdx := 0; stepIdx < MaxBacktrackSteps; stepIdx++ {
		for i, v := range vars {
			if !v.Active {
				continue
			}
			v.Depth += step * direction[i]
			v.Depth = math.Max(MinDepthPerPosition, math.Min(v.UpperBound, v.Depth))
		}

		totalD := 0.0
		for _, v := range vars {
			if v.Active {
				totalD += v.Depth
			}
		}
		if totalD > totalMaxDepth {
			scale := (totalMaxDepth - 1e-6) / totalD
			for _, v := range vars {
				if v.Active {
					v.Depth *= scale
				}
			}
		}

		trialObj, _ := g.augmentedLagrangian(vars, currentFreq, targetFreq, constraints, totalMaxDepth)

		sufficientDecrease := trialObj <= currentObj+BacktrackAlpha*step*dotGradDir

		for i, v := range vars {
			v.Depth -= step * direction[i]
			v.Depth = math.Max(MinDepthPerPosition, math.Min(v.UpperBound, v.Depth))
		}

		if sufficientDecrease || step < 1e-8 {
			return step
		}

		step *= BacktrackBeta
	}

	return step
}

func (g *GradientDescentOptimizer) projectToFeasible(vars []*OptimizationVariable, totalMaxDepth float64) {
	for _, v := range vars {
		v.Depth = math.Max(MinDepthPerPosition, math.Min(v.UpperBound, v.Depth))
	}

	totalDepth := 0.0
	for _, v := range vars {
		if v.Active {
			totalDepth += v.Depth
		}
	}

	if totalDepth > totalMaxDepth {
		ratio := (totalMaxDepth - 1e-6) / totalDepth
		for _, v := range vars {
			if v.Active {
				v.Depth *= ratio
			}
		}
	}
}

func (g *GradientDescentOptimizer) OptimizePitch(
	currentFreq, targetFreq float64,
) *models.PitchCorrection {

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
			Algorithm:            "augmented_lagrangian",
			Status:               "within_tolerance",
		}
	}

	sim := NewFEMSimulator(g.Bell)
	sim.GenerateGrid()

	candidates := g.generateCandidatePositions()

	variables := make([]*OptimizationVariable, 0, len(candidates))
	for _, pos := range candidates {
		sensitivity := sim.CalculateFrequencySensitivity(pos)

		var effectiveScore float64
		if deviationCents > 0 {
			effectiveScore = -sensitivity
		} else {
			effectiveScore = sensitivity
		}

		if effectiveScore > 0.01 {
			variables = append(variables, &OptimizationVariable{
				Position:    pos,
				Depth:       0.05,
				Sensitivity: sensitivity,
				Active:      true,
				LowerBound:  MinDepthPerPosition,
				UpperBound:  MaxDepthPerPosition,
			})
		}
	}

	for i := 0; i < len(variables)-1; i++ {
		for j := i + 1; j < len(variables); j++ {
			scoreI := math.Abs(variables[i].Sensitivity)
			scoreJ := math.Abs(variables[j].Sensitivity)
			if scoreJ > scoreI {
				variables[i], variables[j] = variables[j], variables[i]
			}
		}
	}

	maxVars := 5
	if len(variables) > maxVars {
		for i := maxVars; i < len(variables); i++ {
			variables[i].Active = false
		}
		variables = variables[:maxVars]
	}

	if len(variables) == 0 {
		return &models.PitchCorrection{
			CreatedAt:            time.Now(),
			BellID:               g.Bell.ID,
			CurrentFrequency:     currentFreq,
			TargetFrequency:      targetFreq,
			DeviationCents:       deviationCents,
			RecommendedPositions: []models.CorrectionRecommendation{},
			EstimatedResultFreq:  currentFreq,
			Iterations:           0,
			Algorithm:            "augmented_lagrangian",
			Status:               "no_feasible_direction",
		}
	}

	maxTotalDepth := g.Bell.MaxGrindingDepthMm
	g.projectToFeasible(variables, maxTotalDepth)

	constraints := g.computeConstraints(variables, maxTotalDepth)
	for _, c := range constraints {
		c.Lambda = 0.0
	}

	rho := DefaultRho
	totalIterations := 0
	var prevObj float64 = math.MaxFloat64

	for outerIter := 0; outerIter < g.OuterIterations; outerIter++ {
		consecutiveStationary := 0

		for innerIter := 0; innerIter < g.MaxIterations/5; innerIter++ {
			currentObj, gradients := g.augmentedLagrangian(
				variables, currentFreq, targetFreq, constraints, maxTotalDepth,
			)

			normGrad := 0.0
			allKKT := true
			for i, v := range variables {
				if !v.Active {
					continue
				}
				if !g.kktSatisfied(v, gradients[i]) {
					allKKT = false
				}
				normGrad += gradients[i] * gradients[i]
			}
			normGrad = math.Sqrt(normGrad)

			if allKKT || normGrad < g.ConvergenceFactor || math.Abs(currentObj-prevObj) < 1e-10 {
				consecutiveStationary++
				if consecutiveStationary >= 2 {
					break
				}
			} else {
				consecutiveStationary = 0
			}
			prevObj = currentObj

			for i, v := range variables {
				if !v.Active {
					continue
				}

				grad := gradients[i]
				grad = g.boundaryDamping(v, grad)

				if g.detectOscillation(v) {
					grad *= 0.3
				}

				v.DepthHistory = append(v.DepthHistory, v.Depth)
				v.GradHistory = append(v.GradHistory, grad)
				if len(v.DepthHistory) > OscillationLookback*4 {
					v.DepthHistory = v.DepthHistory[len(v.DepthHistory)-OscillationLookback*4:]
					v.GradHistory = v.GradHistory[len(v.GradHistory)-OscillationLookback*4:]
				}

				if math.Abs(grad) > 1e-12 {
					adaptedLR := g.LearningRate / (1.0 + float64(totalIterations)*0.01)
					update := -adaptedLR * grad
					newDepth := v.Depth + update

					if newDepth < v.LowerBound {
						overShoot := v.LowerBound - newDepth
						if overShoot > BoundaryDampingZone*(v.UpperBound-v.LowerBound) && grad < 0 {
							update = (v.LowerBound - v.Depth) * 0.5
						}
					}
					if newDepth > v.UpperBound {
						overShoot := newDepth - v.UpperBound
						if overShoot > BoundaryDampingZone*(v.UpperBound-v.LowerBound) && grad > 0 {
							update = (v.UpperBound - v.Depth) * 0.5
						}
					}
					v.Depth += update
				}
			}

			g.projectToFeasible(variables, maxTotalDepth)
			totalIterations++
		}

		constraints = g.computeConstraints(variables, maxTotalDepth)

		maxConstraintViolation := 0.0
		allSatisfied := true
		for ci, c := range constraints {
			violation := math.Max(0, c.Value)
			if violation > maxConstraintViolation {
				maxConstraintViolation = violation
			}
			if !c.IsSatisfied {
				allSatisfied = false
			}
			if c.Type == "inequality" {
				lambdaUpdate := rho * c.Value
				if c.Value > -ConstraintTolerance*0.5 {
					constraints[ci].Lambda = math.Max(0, c.Lambda+lambdaUpdate)
				}
			} else {
				constraints[ci].Lambda = c.Lambda + rho*c.Value
			}
		}

		for _, v := range variables {
			if !v.Active {
				continue
			}
			rangeSize := v.UpperBound - v.LowerBound
			if rangeSize <= 0 {
				continue
			}
			if v.Depth < v.LowerBound+ConstraintTolerance*rangeSize {
				v.Depth = v.LowerBound
			}
			if v.Depth > v.UpperBound-ConstraintTolerance*rangeSize {
				v.Depth = v.UpperBound
			}
		}

		if maxConstraintViolation < ConstraintTolerance && allSatisfied {
			break
		}

		rho = math.Min(rho*RhoIncreaseFactor, MaxRho)
		g.Rho = rho
	}

	estimatedFreq := currentFreq
	recommendations := make([]models.CorrectionRecommendation, 0)

	for _, v := range variables {
		if !v.Active || v.Depth < MinDepthPerPosition*2 {
			continue
		}

		freqChange := v.Sensitivity * v.Depth
		if deviationCents > 0 && freqChange > 0 {
			freqChange = -freqChange
		} else if deviationCents < 0 && freqChange < 0 {
			freqChange = -freqChange
		}

		if math.Abs(freqChange) < 0.001 {
			continue
		}

		recommendations = append(recommendations, models.CorrectionRecommendation{
			Position:          v.Position,
			DepthMm:           v.Depth,
			Sensitivity:       v.Sensitivity,
			FrequencyChangeHz: freqChange,
		})

		estimatedFreq += freqChange
	}

	remainingCents := g.frequencyToCents(estimatedFreq, targetFreq)

	status := "recommended"
	switch {
	case math.Abs(remainingCents) <= g.Bell.ToleranceCents:
		status = "achievable"
	default:
		totalDepth := 0.0
		for _, r := range recommendations {
			totalDepth += r.DepthMm
		}
		if totalDepth >= maxTotalDepth*0.98 {
			status = "depth_limit_reached"
		}
	}

	return &models.PitchCorrection{
		CreatedAt:            time.Now(),
		BellID:               g.Bell.ID,
		CurrentFrequency:     currentFreq,
		TargetFrequency:      targetFreq,
		DeviationCents:       deviationCents,
		RecommendedPositions: recommendations,
		EstimatedResultFreq:  estimatedFreq,
		Iterations:           totalIterations,
		Algorithm:            "augmented_lagrangian",
		Status:               status,
	}
}
