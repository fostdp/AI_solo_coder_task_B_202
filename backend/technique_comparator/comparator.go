package technique_comparator

import (
	"math"

	"bianzhong-acoustic-system/models"
	"bianzhong-acoustic-system/simulation"
)

type ProcessType = string

const (
	ProcessGrinding      ProcessType = simulation.ProcessGrinding
	ProcessCastingInlay  ProcessType = simulation.ProcessCastingInlay
	ProcessWeldingRepair ProcessType = simulation.ProcessWeldingRepair
)

type TechniqueComparator struct {
	sim *simulation.FEMSimulator
}

func NewTechniqueComparator(bell *models.Bell) *TechniqueComparator {
	sim := simulation.NewFEMSimulator(bell)
	sim.GenerateGrid()
	return &TechniqueComparator{sim: sim}
}

func (tc *TechniqueComparator) Compare(currentFreq, targetFreq float64) []models.ProcessComparisonResult {
	return tc.sim.CompareTuningProcesses(currentFreq, targetFreq)
}

func (tc *TechniqueComparator) CompareAsync(currentFreq, targetFreq float64) <-chan []models.ProcessComparisonResult {
	resultCh := make(chan []models.ProcessComparisonResult, 1)
	go func() {
		defer close(resultCh)
		resultCh <- tc.sim.CompareTuningProcesses(currentFreq, targetFreq)
	}()
	return resultCh
}

func (tc *TechniqueComparator) BestProcess(currentFreq, targetFreq float64) *models.ProcessComparisonResult {
	results := tc.sim.CompareTuningProcesses(currentFreq, targetFreq)
	if len(results) == 0 {
		return nil
	}

	bestIdx := 0
	for i := range results {
		if results[i].OverallScore > results[bestIdx].OverallScore {
			bestIdx = i
		}
	}
	return &results[bestIdx]
}

func (tc *TechniqueComparator) GetProcessSensitivity(processType string, pos models.GrindingPosition) float64 {
	return tc.sim.CalculateProcessSensitivity(processType, pos)
}

func (tc *TechniqueComparator) GetHarmonicity() float64 {
	return tc.sim.CalculateHarmonicity()
}

func (tc *TechniqueComparator) Reset() {
	tc.sim.Reset()
}

func FindResult(results []models.ProcessComparisonResult, processType string) *models.ProcessComparisonResult {
	for i := range results {
		if results[i].ProcessType == processType {
			return &results[i]
		}
	}
	return nil
}

func AccuracyRank(results []models.ProcessComparisonResult) []models.ProcessComparisonResult {
	sorted := make([]models.ProcessComparisonResult, len(results))
	copy(sorted, results)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if math.Abs(sorted[j].DeviationCents) < math.Abs(sorted[i].DeviationCents) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

func CostBenefitRatio(result models.ProcessComparisonResult) float64 {
	if result.CostScore <= 0 {
		return 0
	}
	return result.OverallScore / result.CostScore
}
