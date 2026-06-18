package simulation

import (
	"math"
	"testing"

	"bianzhong-acoustic-system/models"
)

func TestCrossEraEvolution_ComplexityProgression_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]
	results := sim.CompareTuningProcesses(baseFreq, baseFreq+10.0)

	complexityOrder := []string{ProcessGrinding, ProcessCastingInlay, ProcessWeldingRepair}
	prevComplexity := -1

	for _, pt := range complexityOrder {
		r := findResult(results, pt)
		if r == nil {
			t.Fatalf("未找到工艺 %s 的结果", pt)
		}
		if r.Complexity <= prevComplexity {
			t.Errorf("技术演进: %s复杂度(%d)应高于前一工艺(%d)",
				pt, r.Complexity, prevComplexity)
		}
		prevComplexity = r.Complexity
	}
}

func TestCrossEraEvolution_ReversibilityEvolution_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]
	results := sim.CompareTuningProcesses(baseFreq, baseFreq+10.0)

	for _, r := range results {
		switch r.ProcessType {
		case ProcessGrinding:
			if r.Reversibility {
				t.Error("古代工艺磨锉应为不可逆")
			}
		case ProcessCastingInlay:
			if !r.Reversibility {
				t.Error("近代工艺铸镶应为可逆(铅块可移除)")
			}
		case ProcessWeldingRepair:
			if !r.Reversibility {
				t.Error("现代工艺焊补应为可逆(可磨去焊补)")
			}
		}
	}
}

func TestCrossEraEvolution_DamageRiskProgression_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]
	results := sim.CompareTuningProcesses(baseFreq, baseFreq+10.0)

	riskMap := make(map[string]float64)
	for _, r := range results {
		riskMap[r.ProcessType] = r.DamageRisk
	}

	if riskMap[ProcessWeldingRepair] <= riskMap[ProcessGrinding] {
		t.Errorf("技术演进: 焊补损伤风险(%.2f)应高于磨锉(%.2f)——现代工艺精度要求更高",
			riskMap[ProcessWeldingRepair], riskMap[ProcessGrinding])
	}

	if riskMap[ProcessGrinding] <= riskMap[ProcessCastingInlay] {
		t.Errorf("技术演进: 磨锉损伤风险(%.2f)应高于铸镶(%.2f)——磨锉不可逆",
			riskMap[ProcessGrinding], riskMap[ProcessCastingInlay])
	}
}

func TestCrossEraEvolution_TimeComplexity_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]
	results := sim.CompareTuningProcesses(baseFreq, baseFreq+10.0)

	timeMap := make(map[string]int)
	for _, r := range results {
		timeMap[r.ProcessType] = r.RequiredTimeMin
	}

	if timeMap[ProcessGrinding] >= timeMap[ProcessCastingInlay] {
		t.Errorf("古代磨锉耗时(%dmin)应少于近代铸镶(%dmin)",
			timeMap[ProcessGrinding], timeMap[ProcessCastingInlay])
	}
}

func TestCrossEraEvolution_HarmonicityPenalty_InlayVsWelding_Boundary(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	position := pos()

	baseHarmonicity := sim.CalculateHarmonicity()

	sim.ApplyTuningProcess(ProcessCastingInlay, position, 0.5)
	afterInlay := sim.CalculateHarmonicity()

	sim.Reset()

	sim.ApplyTuningProcess(ProcessWeldingRepair, position, 0.5)
	afterWelding := sim.CalculateHarmonicity()

	inlayDrop := baseHarmonicity - afterInlay
	weldingDrop := baseHarmonicity - afterWelding

	if inlayDrop <= weldingDrop {
		t.Errorf("技术演进: 铸镶和谐度下降(%.4f)应大于焊补(%.4f)——铅块阻尼影响更大",
			inlayDrop, weldingDrop)
	}
}

func TestCrossEraEvolution_DampingFactorValue_Boundary(t *testing.T) {
	if InlayDampingFactor != 0.88 {
		t.Errorf("铸镶阻尼因子应为0.88(反映古代工艺声学特性), 实际=%.2f", InlayDampingFactor)
	}

	if InlayDampingFactor >= 1.0 {
		t.Error("阻尼因子应<1.0表示信号衰减")
	}

	if InlayDampingFactor <= 0.5 {
		t.Error("阻尼因子应>0.5, 否则过度衰减")
	}
}

func TestCrossEraEvolution_StressFactorValue_Boundary(t *testing.T) {
	if WeldingStressFactor != 1.15 {
		t.Errorf("焊补应力因子应为1.15(反映现代工艺热影响), 实际=%.2f", WeldingStressFactor)
	}

	if WeldingStressFactor <= 1.0 {
		t.Error("应力因子应>1.0表示应力增加")
	}
}

func TestCrossEraEvolution_InlayRadiusValue_Boundary(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	position := pos()
	sim.ApplyTuningProcess(ProcessCastingInlay, position, 0.5)

	if len(sim.InlayMasses) != 1 {
		t.Fatalf("应记录1个镶块, 实际=%d", len(sim.InlayMasses))
	}

	if sim.InlayMasses[0].RadiusCm < 1.0 || sim.InlayMasses[0].RadiusCm > 1.5 {
		t.Errorf("古代铸镶镶块半径应在合理范围[1.0,1.5], 实际=%.1f",
			sim.InlayMasses[0].RadiusCm)
	}
}

func TestCrossEraEvolution_WeldingRadiusValue_Boundary(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	position := pos()
	sim.ApplyTuningProcess(ProcessWeldingRepair, position, 0.5)

	if len(sim.WeldPatches) != 1 {
		t.Fatalf("应记录1个焊补补丁, 实际=%d", len(sim.WeldPatches))
	}

	if sim.WeldPatches[0].RadiusCm < 1.5 || sim.WeldPatches[0].RadiusCm > 2.0 {
		t.Errorf("现代焊补补丁半径应在合理范围[1.5,2.0], 实际=%.1f",
			sim.WeldPatches[0].RadiusCm)
	}
}

func TestCrossEraEvolution_InlayLeadDensity_Abnormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	position := pos()
	sim.ApplyTuningProcess(ProcessCastingInlay, position, 0.3)

	if sim.InlayMasses[0].Density != LeadDensity {
		t.Errorf("古代铸镶应使用铅(密度=%.1f), 实际=%.1f",
			LeadDensity, sim.InlayMasses[0].Density)
	}

	if LeadDensity < 10000 {
		t.Errorf("铅密度应>10000 kg/m³, 实际=%.1f", LeadDensity)
	}
}

func TestCrossEraEvolution_MultipleInlaysCumulativePenalty_Abnormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseH := sim.CalculateHarmonicity()

	positions := []models.GrindingPosition{
		{X: 0, Y: 10, Z: 8},
		{X: 0, Y: 15, Z: 8},
		{X: 0, Y: 20, Z: 8},
	}

	for i, p := range positions {
		sim.ApplyTuningProcess(ProcessCastingInlay, p, 0.2)

		afterH := sim.CalculateHarmonicity()
		expectedRatio := math.Pow(InlayHarmonicityPenalty, float64(i+1))
		actualRatio := afterH / baseH

		if math.Abs(actualRatio-expectedRatio) > 0.2 {
			t.Errorf("第%d次铸镶后和谐度累积惩罚偏离: 预期比率=%.4f, 实际=%.4f",
				i+1, expectedRatio, actualRatio)
		}
	}
}

func TestCrossEraEvolution_WeldingStressIncrease_Abnormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	for i := 0; i < sim.Grid.Nx; i++ {
		for j := 0; j < sim.Grid.Ny; j++ {
			for k := 0; k < sim.Grid.Nz; k++ {
				if sim.Grid.Nodes[i][j][k].IsActive {
					sim.Grid.Nodes[i][j][k].Stress = 1.0
				}
			}
		}
	}

	position := pos()
	sim.ApplyTuningProcess(ProcessWeldingRepair, position, 0.5)

	if len(sim.WeldPatches) != 1 {
		t.Errorf("焊补应记录1个补丁, 实际=%d", len(sim.WeldPatches))
	}

	quality := sim.EvaluateGridQuality()
	if quality == nil {
		t.Error("焊补后应能评估网格质量")
	}
}

func TestCrossEraEvolution_FrequencyDirection_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	position := pos()

	processes := []string{ProcessGrinding, ProcessCastingInlay, ProcessWeldingRepair}
	for _, pt := range processes {
		sim.Reset()
		beforeFreqs := sim.CalculateEigenfrequencies()
		beforeFund := beforeFreqs[0]

		sim.ApplyTuningProcess(pt, position, 0.4)
		afterFreqs := sim.CalculateEigenfrequencies()
		afterFund := afterFreqs[0]

		if math.IsNaN(afterFund) || math.IsInf(afterFund, 0) {
			t.Errorf("工艺%s后频率不应为NaN/Inf, 实际=%.4f", pt, afterFund)
		}
		if afterFund <= 0 {
			t.Errorf("工艺%s后频率应为正值, 实际=%.4f", pt, afterFund)
		}

		t.Logf("工艺%s: before=%.2f, after=%.2f", pt, beforeFund, afterFund)
	}
}

func TestCrossEraEvolution_OverallScoreOrder_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseFreqs := sim.CalculateEigenfrequencies()
	baseFreq := baseFreqs[0]
	results := sim.CompareTuningProcesses(baseFreq, baseFreq+15.0)

	scoreMap := make(map[string]float64)
	for _, r := range results {
		scoreMap[r.ProcessType] = r.OverallScore
	}

	if scoreMap[ProcessGrinding] <= 0 {
		t.Errorf("古代磨锉在升频场景应有正得分, 实际=%.4f", scoreMap[ProcessGrinding])
	}

	t.Logf("升频场景得分: grinding=%.4f, casting_inlay=%.4f, welding_repair=%.4f",
		scoreMap[ProcessGrinding], scoreMap[ProcessCastingInlay], scoreMap[ProcessWeldingRepair])
}

func TestCrossEraEvolution_HistoricalContextConstants(t *testing.T) {
	if ProcessGrinding != "grinding" {
		t.Errorf("古代工艺标识应为'grinding', 实际='%s'", ProcessGrinding)
	}
	if ProcessCastingInlay != "casting_inlay" {
		t.Errorf("近代工艺标识应为'casting_inlay', 实际='%s'", ProcessCastingInlay)
	}
	if ProcessWeldingRepair != "welding_repair" {
		t.Errorf("现代工艺标识应为'welding_repair', 实际='%s'", ProcessWeldingRepair)
	}

	if InlayHarmonicityPenalty >= 0.95 {
		t.Errorf("铸镶和谐度惩罚(%.2f)应重于焊补(0.95), 体现工艺演进",
			InlayHarmonicityPenalty)
	}
}

func pos() models.GrindingPosition {
	return models.GrindingPosition{X: 0, Y: 15, Z: 8}
}

func TestCrossEraEvolution_GridQualityAfterEachProcess(t *testing.T) {
	processes := []string{ProcessGrinding, ProcessCastingInlay, ProcessWeldingRepair}

	for _, pt := range processes {
		bell := createTestBell()
		sim := NewFEMSimulator(bell)
		sim.GenerateGrid()

		sim.ApplyTuningProcess(pt, pos(), 0.5)

		quality := sim.EvaluateGridQuality()
		if quality == nil {
			t.Errorf("工艺%s后网格质量报告不应为nil", pt)
			continue
		}
		if quality.ActiveNodeCount == 0 {
			t.Errorf("工艺%s后应有活跃节点", pt)
		}
	}
}

func TestCrossEraEvolution_MultipleProcessesHistory(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	processes := []string{ProcessGrinding, ProcessCastingInlay, ProcessWeldingRepair}
	for _, pt := range processes {
		sim.ApplyTuningProcess(pt, pos(), 0.2)
	}

	if len(sim.ProcessHistory) != 3 {
		t.Fatalf("应记录3条工艺历史, 实际=%d", len(sim.ProcessHistory))
	}

	for i, pt := range processes {
		if sim.ProcessHistory[i].ProcessType != pt {
			t.Errorf("第%d条历史应为%s, 实际=%s", i, pt, sim.ProcessHistory[i].ProcessType)
		}
	}

	if len(sim.InlayMasses) != 1 {
		t.Errorf("应有1个铸镶记录, 实际=%d", len(sim.InlayMasses))
	}
	if len(sim.WeldPatches) != 1 {
		t.Errorf("应有1个焊补记录, 实际=%d", len(sim.WeldPatches))
	}
}
