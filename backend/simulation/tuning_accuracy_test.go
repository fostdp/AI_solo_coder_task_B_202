package simulation

import (
	"math"
	"testing"

	"bianzhong-acoustic-system/models"
)

func TestTuningAccuracy_Grinding_PrecisionNormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]

	targetFreq := baseFreq + 15.0
	results := sim.CompareTuningProcesses(baseFreq, targetFreq)

	for _, r := range results {
		if r.ProcessType == ProcessGrinding {
			if r.DeviationCents > 50 {
				t.Errorf("磨锉调音精度超出范围: deviation=%.2f cents, 应<50", r.DeviationCents)
			}
			if math.Abs(r.EstimatedFreq-targetFreq)/targetFreq > 0.03 {
				t.Errorf("磨锉估计频率偏差过大: estimated=%.2f, target=%.2f", r.EstimatedFreq, targetFreq)
			}
		}
	}
}

func TestTuningAccuracy_CastingInlay_PrecisionNormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]

	targetFreq := baseFreq - 10.0
	results := sim.CompareTuningProcesses(baseFreq, targetFreq)

	for _, r := range results {
		if r.ProcessType == ProcessCastingInlay {
			if r.DeviationCents > 60 {
				t.Errorf("铸镶调音精度超出范围: deviation=%.2f cents, 应<60", r.DeviationCents)
			}
		}
	}
}

func TestTuningAccuracy_WeldingRepair_PrecisionNormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]

	targetFreq := baseFreq - 8.0
	results := sim.CompareTuningProcesses(baseFreq, targetFreq)

	for _, r := range results {
		if r.ProcessType == ProcessWeldingRepair {
			if r.DeviationCents > 70 {
				t.Errorf("焊补调音精度超出范围: deviation=%.2f cents, 应<70", r.DeviationCents)
			}
		}
	}
}

func TestTuningAccuracy_SmallDelta_Boundary(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]

	targetFreq := baseFreq + 0.5
	results := sim.CompareTuningProcesses(baseFreq, targetFreq)

	if len(results) != 3 {
		t.Fatalf("微小频率差仍应返回3种工艺结果, 实际=%d", len(results))
	}

	for _, r := range results {
		if math.IsNaN(r.OverallScore) || math.IsInf(r.OverallScore, 0) {
			t.Errorf("工艺%s在微小调整下OverallScore不应为NaN/Inf", r.ProcessType)
		}
		if math.IsNaN(r.DeviationCents) || math.IsInf(r.DeviationCents, 0) {
			t.Errorf("工艺%s在微小调整下DeviationCents不应为NaN/Inf", r.ProcessType)
		}
	}
}

func TestTuningAccuracy_LargeDelta_Boundary(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]

	targetFreq := baseFreq * 1.5
	results := sim.CompareTuningProcesses(baseFreq, targetFreq)

	if len(results) != 3 {
		t.Fatalf("大频率差仍应返回3种工艺结果, 实际=%d", len(results))
	}

	for _, r := range results {
		if r.EstimatedFreq <= 0 {
			t.Errorf("工艺%s估计频率应为正值, 实际=%.2f", r.ProcessType, r.EstimatedFreq)
		}
		if r.OverallScore < 0 || r.OverallScore > 1 {
			t.Errorf("工艺%s OverallScore应在[0,1], 实际=%.4f", r.ProcessType, r.OverallScore)
		}
	}
}

func TestTuningAccuracy_ZeroDelta_Boundary(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]

	results := sim.CompareTuningProcesses(baseFreq, baseFreq)

	for _, r := range results {
		if math.IsNaN(r.FreqDeltaHz) || math.IsInf(r.FreqDeltaHz, 0) {
			t.Errorf("零目标差时%s频率变化不应为NaN/Inf, 实际=%.4f", r.ProcessType, r.FreqDeltaHz)
		}
		if r.OverallScore < 0 || r.OverallScore > 1 {
			t.Errorf("零目标差时%s OverallScore应在[0,1], 实际=%.4f", r.ProcessType, r.OverallScore)
		}
	}
}

func TestTuningAccuracy_NegativeDelta_Abnormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]

	targetFreq := baseFreq - 50.0
	results := sim.CompareTuningProcesses(baseFreq, targetFreq)

	grindingResult := findResult(results, ProcessGrinding)
	if grindingResult == nil {
		t.Fatal("应返回磨锉结果")
	}

	if math.IsNaN(grindingResult.OverallScore) || math.IsInf(grindingResult.OverallScore, 0) {
		t.Errorf("磨锉在降频场景下OverallScore不应为NaN/Inf, 实际=%.4f", grindingResult.OverallScore)
	}
	if grindingResult.OverallScore < 0 || grindingResult.OverallScore > 1 {
		t.Errorf("磨锉OverallScore应在[0,1]范围, 实际=%.4f", grindingResult.OverallScore)
	}
}

func TestTuningAccuracy_ExtremeNegativeDelta_Abnormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]

	targetFreq := baseFreq * 0.1
	results := sim.CompareTuningProcesses(baseFreq, targetFreq)

	for _, r := range results {
		if math.IsNaN(r.OverallScore) || math.IsInf(r.OverallScore, 0) {
			t.Errorf("极端降频目标下%s OverallScore不应为NaN/Inf", r.ProcessType)
		}
	}
}

func TestTuningAccuracy_InvalidTargetFreq_Abnormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	results := sim.CompareTuningProcesses(440, -100)

	for _, r := range results {
		if math.IsNaN(r.EstimatedFreq) || math.IsInf(r.EstimatedFreq, 0) {
			t.Errorf("负目标频率下%s EstimatedFreq不应为NaN/Inf", r.ProcessType)
		}
	}
}

func TestTuningAccuracy_HarmonicityPreservation(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseHarmonicity := sim.CalculateHarmonicity()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]
	targetFreq := baseFreq + 10.0

	results := sim.CompareTuningProcesses(baseFreq, targetFreq)

	for _, r := range results {
		if r.ProcessType == ProcessGrinding {
			drop := baseHarmonicity - r.Harmonicity
			if drop > 0.15 {
				t.Errorf("磨锉后和谐度下降过多: base=%.4f, after=%.4f, drop=%.4f",
					baseHarmonicity, r.Harmonicity, drop)
			}
		}
	}
}

func TestTuningAccuracy_MultipleComparisons_Consistency(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]
	targetFreq := baseFreq + 12.0

	results1 := sim.CompareTuningProcesses(baseFreq, targetFreq)
	results2 := sim.CompareTuningProcesses(baseFreq, targetFreq)

	if len(results1) != len(results2) {
		t.Fatalf("两次比较结果数量不一致: %d vs %d", len(results1), len(results2))
	}

	for i := range results1 {
		if results1[i].ProcessType != results2[i].ProcessType {
			t.Errorf("第%d个结果工艺类型不一致: %s vs %s",
				i, results1[i].ProcessType, results2[i].ProcessType)
		}
		if results1[i].OverallScore < 0 || results1[i].OverallScore > 1 {
			t.Errorf("%s 第一次OverallScore超出范围: %.4f", results1[i].ProcessType, results1[i].OverallScore)
		}
		if results2[i].OverallScore < 0 || results2[i].OverallScore > 1 {
			t.Errorf("%s 第二次OverallScore超出范围: %.4f", results2[i].ProcessType, results2[i].OverallScore)
		}
		if math.IsNaN(results1[i].OverallScore) || math.IsInf(results1[i].OverallScore, 0) {
			t.Errorf("%s 第一次OverallScore为NaN/Inf", results1[i].ProcessType)
		}
		if math.IsNaN(results2[i].OverallScore) || math.IsInf(results2[i].OverallScore, 0) {
			t.Errorf("%s 第二次OverallScore为NaN/Inf", results2[i].ProcessType)
		}
	}
}

func TestTuningAccuracy_DeviationCentsCalculation(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]
	targetFreq := baseFreq + 20.0

	results := sim.CompareTuningProcesses(baseFreq, targetFreq)

	for _, r := range results {
		expectedCents := 1200.0 * math.Log2(r.EstimatedFreq/targetFreq)
		if math.Abs(r.DeviationCents-expectedCents) > 1.0 {
			t.Errorf("%s 音分计算错误: 实际=%.2f, 预期=%.2f",
				r.ProcessType, r.DeviationCents, expectedCents)
		}
	}
}

func findResult(results []models.ProcessComparisonResult, processType string) *models.ProcessComparisonResult {
	for i := range results {
		if results[i].ProcessType == processType {
			return &results[i]
		}
	}
	return nil
}

func TestTuningAccuracy_BestProcessSelection(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]
	targetFreq := baseFreq + 25.0

	results := sim.CompareTuningProcesses(baseFreq, targetFreq)

	bestScore := -1.0
	bestProcess := ""
	for _, r := range results {
		if r.OverallScore > bestScore {
			bestScore = r.OverallScore
			bestProcess = r.ProcessType
		}
	}

	if bestProcess == "" {
		t.Error("应选出最佳工艺")
	}
	if bestScore < 0 {
		t.Errorf("最佳分数应为非负, 实际=%.4f", bestScore)
	}

	t.Logf("升频场景最佳工艺: %s (score=%.4f)", bestProcess, bestScore)
}

func TestTuningAccuracy_GridPreservationAfterComparison(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFund := beforeFreqs[0]

	sim.CompareTuningProcesses(beforeFund, beforeFund+20.0)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFund := afterFreqs[0]

	relativeDiff := math.Abs(afterFund-beforeFund) / beforeFund
	if relativeDiff > 0.02 {
		t.Errorf("工艺对比后网格应恢复: before=%.2f, after=%.2f, 偏差=%.2f%%",
			beforeFund, afterFund, relativeDiff*100)
	}
}

func TestTuningAccuracy_CostScoreConsistency(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]
	results := sim.CompareTuningProcesses(baseFreq, baseFreq+10.0)

	costMap := make(map[string]float64)
	for _, r := range results {
		costMap[r.ProcessType] = r.CostScore
	}

	if costMap[ProcessGrinding] >= costMap[ProcessCastingInlay] {
		t.Errorf("磨锉成本应低于铸镶: grinding=%.2f, casting_inlay=%.2f",
			costMap[ProcessGrinding], costMap[ProcessCastingInlay])
	}
	if costMap[ProcessCastingInlay] >= costMap[ProcessWeldingRepair] {
		t.Errorf("铸镶成本应低于焊补: casting_inlay=%.2f, welding_repair=%.2f",
			costMap[ProcessCastingInlay], costMap[ProcessWeldingRepair])
	}
}
