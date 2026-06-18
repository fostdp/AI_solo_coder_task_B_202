package era_comparator

import (
	"bianzhong-acoustic-system/models"
	"bianzhong-acoustic-system/simulation"
)

type EraName = string

const (
	EraGrinding      EraName = simulation.ProcessGrinding
	EraCastingInlay  EraName = simulation.ProcessCastingInlay
	EraWeldingRepair EraName = simulation.ProcessWeldingRepair
)

type EraMetrics struct {
	EraName            string  `json:"era_name"`
	ComplexityScore    float64 `json:"complexity_score"`
	ReversibilityScore float64 `json:"reversibility_score"`
	DamageRiskScore    float64 `json:"damage_risk_score"`
	EstimatedHours     float64 `json:"estimated_hours"`
	HarmonicityImpact  float64 `json:"harmonicity_impact"`
	HistoricalEra      string  `json:"historical_era"`
}

type EraComparator struct {
	sim *simulation.FEMSimulator
}

func NewEraComparator(bell *models.Bell) *EraComparator {
	sim := simulation.NewFEMSimulator(bell)
	sim.GenerateGrid()
	return &EraComparator{sim: sim}
}

func (ec *EraComparator) CompareEras(currentFreq, targetFreq float64) []EraMetrics {
	results := ec.sim.CompareTuningProcesses(currentFreq, targetFreq)
	metrics := make([]EraMetrics, 0, len(results))

	eraMap := map[string]string{
		simulation.ProcessGrinding:     "ancient",
		simulation.ProcessCastingInlay: "ancient",
		simulation.ProcessWeldingRepair: "modern",
	}

	for _, r := range results {
		complexityNorm := 1.0 - float64(r.Complexity)/10.0
		if complexityNorm < 0 {
			complexityNorm = 0
		}
		reversibilityFloat := 0.0
		if r.Reversibility {
			reversibilityFloat = 1.0
		}
		m := EraMetrics{
			EraName:            r.ProcessType,
			ComplexityScore:    complexityNorm,
			ReversibilityScore: reversibilityFloat,
			DamageRiskScore:    1.0 - r.DamageRisk,
			EstimatedHours:     float64(r.RequiredTimeMin) / 60.0,
			HarmonicityImpact:  r.Harmonicity,
			HistoricalEra:      eraMap[r.ProcessType],
		}
		metrics = append(metrics, m)
	}

	return metrics
}

func (ec *EraComparator) CompareErasAsync(currentFreq, targetFreq float64) <-chan []EraMetrics {
	resultCh := make(chan []EraMetrics, 1)
	go func() {
		defer close(resultCh)
		resultCh <- ec.CompareEras(currentFreq, targetFreq)
	}()
	return resultCh
}

func (ec *EraComparator) GetEraDetail(era string, currentFreq, targetFreq float64) *EraMetrics {
	all := ec.CompareEras(currentFreq, targetFreq)
	for i := range all {
		if all[i].EraName == era {
			return &all[i]
		}
	}
	return nil
}

func (ec *EraComparator) EvolutionIndex() float64 {
	results := ec.sim.CompareTuningProcesses(400, 440)
	if len(results) == 0 {
		return 0
	}

	bestScore := 0.0
	worstScore := 1.0
	for _, r := range results {
		if r.OverallScore > bestScore {
			bestScore = r.OverallScore
		}
		if r.OverallScore < worstScore {
			worstScore = r.OverallScore
		}
	}

	if worstScore <= 0 {
		return bestScore * 100
	}
	return (bestScore - worstScore) / worstScore * 100
}

func (ec *EraComparator) TechnologyProgressScore() map[string]float64 {
	scores := make(map[string]float64)
	eras := []string{simulation.ProcessGrinding, simulation.ProcessCastingInlay, simulation.ProcessWeldingRepair}

	results := ec.sim.CompareTuningProcesses(400, 440)
	resultMap := make(map[string]models.ProcessComparisonResult)
	for _, r := range results {
		resultMap[r.ProcessType] = r
	}

	for _, era := range eras {
		if r, ok := resultMap[era]; ok {
			score := r.OverallScore * 100
			scores[era] = score
		} else {
			scores[era] = 0
		}
	}

	return scores
}

func (ec *EraComparator) Reset() {
	ec.sim.Reset()
}
