package simulation

import (
	"math"
	"testing"

	"bianzhong-acoustic-system/models"
)

func createTestBell() *models.Bell {
	return &models.Bell{
		ID: 1, Name: "TestBell", MassKg: 2.5, HeightCm: 30.0,
		DiameterCm: 20.0, ThicknessMm: 8.0, TargetFrequency: 440.0,
		ToleranceCents: 10.0, MaxGrindingDepthMm: 2.0,
	}
}

func TestApplyTuningProcess_Grinding_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFund := beforeFreqs[0]

	sim.ApplyTuningProcess(ProcessGrinding, pos, 0.5)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFund := afterFreqs[0]

	if afterFund <= beforeFund {
		t.Errorf("磨锉后基频应升高: before=%.2f, after=%.2f", beforeFund, afterFund)
	}

	if len(sim.ProcessHistory) != 1 {
		t.Errorf("应有1条工艺记录, 实际=%d", len(sim.ProcessHistory))
	}

	if sim.ProcessHistory[0].ProcessType != ProcessGrinding {
		t.Errorf("工艺类型应为grinding, 实际=%s", sim.ProcessHistory[0].ProcessType)
	}
}

func TestApplyTuningProcess_CastingInlay_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}

	sim.ApplyTuningProcess(ProcessCastingInlay, pos, 0.5)

	if len(sim.InlayMasses) != 1 {
		t.Errorf("应有1个镶块记录, 实际=%d", len(sim.InlayMasses))
	}

	if sim.InlayMasses[0].Density != LeadDensity {
		t.Errorf("镶块密度应为%.1f, 实际=%.1f", LeadDensity, sim.InlayMasses[0].Density)
	}

	if sim.InlayMasses[0].RadiusCm != 1.2 {
		t.Errorf("镶块半径应为1.2cm, 实际=%.1f", sim.InlayMasses[0].RadiusCm)
	}

	thicknessIncreased := false
	for i := 0; i < sim.Grid.Nx; i++ {
		for j := 0; j < sim.Grid.Ny; j++ {
			for k := 0; k < sim.Grid.Nz; k++ {
				node := sim.Grid.Nodes[i][j][k]
				if node.IsActive && node.Thickness > node.OriginalThickness {
					thicknessIncreased = true
					break
				}
			}
			if thicknessIncreased {
				break
			}
		}
		if thicknessIncreased {
			break
		}
	}
	if !thicknessIncreased {
		t.Error("铸镶后应有节点厚度增加")
	}
}

func TestApplyTuningProcess_WeldingRepair_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}

	sim.ApplyTuningProcess(ProcessWeldingRepair, pos, 0.5)

	if len(sim.WeldPatches) != 1 {
		t.Errorf("应有1个焊补记录, 实际=%d", len(sim.WeldPatches))
	}

	if sim.WeldPatches[0].RadiusCm != 1.8 {
		t.Errorf("焊补半径应为1.8cm, 实际=%.1f", sim.WeldPatches[0].RadiusCm)
	}

	thicknessIncreased := false
	for i := 0; i < sim.Grid.Nx; i++ {
		for j := 0; j < sim.Grid.Ny; j++ {
			for k := 0; k < sim.Grid.Nz; k++ {
				node := sim.Grid.Nodes[i][j][k]
				if node.IsActive && node.Thickness > node.OriginalThickness {
					thicknessIncreased = true
					break
				}
			}
			if thicknessIncreased {
				break
			}
		}
		if thicknessIncreased {
			break
		}
	}
	if !thicknessIncreased {
		t.Error("焊补后应有节点厚度增加")
	}
}

func TestApplyTuningProcess_InvalidProcessType(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess("unknown_process", pos, 0.3)

	if len(sim.ProcessHistory) != 1 {
		t.Errorf("未知工艺类型仍应记录工艺历史, 实际=%d", len(sim.ProcessHistory))
	}

	if sim.ProcessHistory[0].ProcessType != "unknown_process" {
		t.Errorf("记录应保留原始工艺类型, 实际=%s", sim.ProcessHistory[0].ProcessType)
	}

	thicknessChanged := false
	for i := 0; i < sim.Grid.Nx && !thicknessChanged; i++ {
		for j := 0; j < sim.Grid.Ny && !thicknessChanged; j++ {
			for k := 0; k < sim.Grid.Nz; k++ {
				node := sim.Grid.Nodes[i][j][k]
				if node.IsActive && node.Thickness != node.OriginalThickness {
					thicknessChanged = true
					break
				}
			}
		}
	}
	if !thicknessChanged {
		t.Error("未知工艺类型应回退到磨锉行为(改变网格厚度)")
	}
}

func TestCompareTuningProcesses_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	currentFreq := sim.CalculateEigenfrequencies()[0]
	targetFreq := currentFreq + 20.0

	results := sim.CompareTuningProcesses(currentFreq, targetFreq)

	if len(results) != 3 {
		t.Errorf("应返回3种工艺对比结果, 实际=%d", len(results))
	}

	processTypes := make(map[string]bool)
	for _, r := range results {
		processTypes[r.ProcessType] = true
	}

	if !processTypes[ProcessGrinding] || !processTypes[ProcessCastingInlay] || !processTypes[ProcessWeldingRepair] {
		t.Errorf("应包含三种工艺类型: grinding, casting_inlay, welding_repair")
	}
}

func TestCompareTuningProcesses_ComplexityOrder(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	currentFreq := sim.CalculateEigenfrequencies()[0]
	targetFreq := currentFreq + 10.0

	results := sim.CompareTuningProcesses(currentFreq, targetFreq)

	complexityMap := make(map[string]int)
	for _, r := range results {
		complexityMap[r.ProcessType] = r.Complexity
	}

	if complexityMap[ProcessGrinding] >= complexityMap[ProcessCastingInlay] {
		t.Errorf("磨锉复杂度应低于铸镶: grinding=%d, casting_inlay=%d",
			complexityMap[ProcessGrinding], complexityMap[ProcessCastingInlay])
	}

	if complexityMap[ProcessCastingInlay] >= complexityMap[ProcessWeldingRepair] {
		t.Errorf("铸镶复杂度应低于焊补: casting_inlay=%d, welding_repair=%d",
			complexityMap[ProcessCastingInlay], complexityMap[ProcessWeldingRepair])
	}
}

func TestCompareTuningProcesses_Reversibility(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	currentFreq := sim.CalculateEigenfrequencies()[0]
	targetFreq := currentFreq + 10.0

	results := sim.CompareTuningProcesses(currentFreq, targetFreq)

	for _, r := range results {
		switch r.ProcessType {
		case ProcessGrinding:
			if r.Reversibility {
				t.Error("磨锉应为不可逆工艺")
			}
		case ProcessCastingInlay:
			if !r.Reversibility {
				t.Error("铸镶应为可逆工艺")
			}
		case ProcessWeldingRepair:
			if !r.Reversibility {
				t.Error("焊补应为可逆工艺")
			}
		}
	}
}

func TestCompareTuningProcesses_DamageRisk(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	currentFreq := sim.CalculateEigenfrequencies()[0]
	targetFreq := currentFreq + 10.0

	results := sim.CompareTuningProcesses(currentFreq, targetFreq)

	riskMap := make(map[string]float64)
	for _, r := range results {
		riskMap[r.ProcessType] = r.DamageRisk
	}

	if riskMap[ProcessWeldingRepair] <= riskMap[ProcessGrinding] {
		t.Errorf("焊补损伤风险应高于磨锉: welding=%.2f, grinding=%.2f",
			riskMap[ProcessWeldingRepair], riskMap[ProcessGrinding])
	}

	if riskMap[ProcessGrinding] <= riskMap[ProcessCastingInlay] {
		t.Errorf("磨锉损伤风险应高于铸镶: grinding=%.2f, casting_inlay=%.2f",
			riskMap[ProcessGrinding], riskMap[ProcessCastingInlay])
	}
}

func TestCalculateProcessSensitivity_Grinding(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sensitivity := sim.CalculateProcessSensitivity(ProcessGrinding, pos)

	if sensitivity <= 0 {
		t.Errorf("磨锉灵敏度应为正值(升频), 实际=%.4f", sensitivity)
	}
}

func TestCalculateProcessSensitivity_CastingInlay(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sensitivity := sim.CalculateProcessSensitivity(ProcessCastingInlay, pos)

	if sensitivity == 0 {
		t.Errorf("铸镶灵敏度应非零, 实际=%.4f", sensitivity)
	}

	if math.IsNaN(sensitivity) || math.IsInf(sensitivity, 0) {
		t.Errorf("铸镶灵敏度不应为NaN或Inf, 实际=%.4f", sensitivity)
	}
}

func TestCalculateProcessSensitivity_WeldingRepair(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sensitivity := sim.CalculateProcessSensitivity(ProcessWeldingRepair, pos)

	if sensitivity == 0 {
		t.Errorf("焊补灵敏度应非零, 实际=%.4f", sensitivity)
	}

	if math.IsNaN(sensitivity) || math.IsInf(sensitivity, 0) {
		t.Errorf("焊补灵敏度不应为NaN或Inf, 实际=%.4f", sensitivity)
	}
}

func TestCalculateHarmonicity_InitialBell(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	harmonicity := sim.CalculateHarmonicity()

	if harmonicity < 0 || harmonicity > 1 {
		t.Errorf("和谐度应在[0,1]范围内, 实际=%.4f", harmonicity)
	}

	if harmonicity <= 0 {
		t.Errorf("初始编钟和谐度应大于0, 实际=%.4f", harmonicity)
	}
}

func TestCalculateHarmonicity_AfterGrinding(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseHarmonicity := sim.CalculateHarmonicity()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessGrinding, pos, 0.3)

	afterHarmonicity := sim.CalculateHarmonicity()

	if afterHarmonicity < 0 || afterHarmonicity > 1 {
		t.Errorf("磨锉后和谐度应在[0,1]范围内, 实际=%.4f", afterHarmonicity)
	}

	if len(sim.InlayMasses) != 0 || len(sim.WeldPatches) != 0 {
		t.Error("磨锉不应产生镶块或焊补记录")
	}

	t.Logf("磨锉: base=%.4f, after=%.4f", baseHarmonicity, afterHarmonicity)
}

func TestCalculateHarmonicity_InlayPenalty(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseHarmonicity := sim.CalculateHarmonicity()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessCastingInlay, pos, 0.3)

	afterHarmonicity := sim.CalculateHarmonicity()

	expectedRatio := InlayHarmonicityPenalty
	actualRatio := afterHarmonicity / baseHarmonicity

	if math.Abs(actualRatio-expectedRatio) > 0.15 {
		t.Errorf("铸镶和谐度惩罚因子应接近%.2f, 实际比率=%.4f (base=%.4f, after=%.4f)",
			expectedRatio, actualRatio, baseHarmonicity, afterHarmonicity)
	}
}

func TestCalculateHarmonicity_WeldingPenalty(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseHarmonicity := sim.CalculateHarmonicity()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessWeldingRepair, pos, 0.3)

	afterHarmonicity := sim.CalculateHarmonicity()

	expectedRatio := 0.95
	actualRatio := afterHarmonicity / baseHarmonicity

	if math.Abs(actualRatio-expectedRatio) > 0.15 {
		t.Errorf("焊补和谐度惩罚因子应接近%.2f, 实际比率=%.4f (base=%.4f, after=%.4f)",
			expectedRatio, actualRatio, baseHarmonicity, afterHarmonicity)
	}
}

func TestCalculateHarmonicity_MultipleInlayPenalties(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	baseHarmonicity := sim.CalculateHarmonicity()

	positions := []models.GrindingPosition{
		{X: 0, Y: 10, Z: 8},
		{X: 0, Y: 15, Z: 8},
	}

	for _, pos := range positions {
		sim.ApplyTuningProcess(ProcessCastingInlay, pos, 0.2)
	}

	afterHarmonicity := sim.CalculateHarmonicity()
	expectedRatio := math.Pow(InlayHarmonicityPenalty, 2)
	actualRatio := afterHarmonicity / baseHarmonicity

	if math.Abs(actualRatio-expectedRatio) > 0.2 {
		t.Errorf("2次铸镶后惩罚因子应接近%.4f, 实际=%.4f", expectedRatio, actualRatio)
	}
}

func TestCrossEra_GrindingPhysicalModel(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFund := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessGrinding, pos, 0.5)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFund := afterFreqs[0]

	if afterFund <= beforeFund {
		t.Errorf("磨锉物理模型: 减薄→升频, 实际 before=%.2f after=%.2f", beforeFund, afterFund)
	}

	if len(sim.ProcessHistory) != 1 {
		t.Errorf("磨锉应记录1条工艺历史, 实际=%d", len(sim.ProcessHistory))
	}
}

func TestCrossEra_CastingInlayPhysicalModel(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessCastingInlay, pos, 0.5)

	if len(sim.InlayMasses) != 1 {
		t.Errorf("铸镶应记录1个镶块, 实际=%d", len(sim.InlayMasses))
	}

	if sim.InlayMasses[0].Density != LeadDensity {
		t.Errorf("镶块密度应为铅密度%.1f, 实际=%.1f", LeadDensity, sim.InlayMasses[0].Density)
	}

	thicknessIncreased := false
	for i := 0; i < sim.Grid.Nx && !thicknessIncreased; i++ {
		for j := 0; j < sim.Grid.Ny && !thicknessIncreased; j++ {
			for k := 0; k < sim.Grid.Nz; k++ {
				node := sim.Grid.Nodes[i][j][k]
				if node.IsActive && node.Thickness > node.OriginalThickness {
					thicknessIncreased = true
					break
				}
			}
		}
	}
	if !thicknessIncreased {
		t.Error("铸镶物理模型: 应增厚钟壁")
	}

	beforeHarmonicity := sim.CalculateHarmonicity()
	sim.ApplyTuningProcess(ProcessCastingInlay, models.GrindingPosition{X: 1, Y: 10, Z: 7}, 0.3)
	afterHarmonicity := sim.CalculateHarmonicity()
	if afterHarmonicity >= beforeHarmonicity {
		t.Errorf("铸镶物理模型: 和谐度应受惩罚降低, before=%.4f, after=%.4f", beforeHarmonicity, afterHarmonicity)
	}
}

func TestCrossEra_WeldingRepairPhysicalModel(t *testing.T) {
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

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessWeldingRepair, pos, 0.5)

	if len(sim.WeldPatches) != 1 {
		t.Errorf("焊补应记录1个焊补补丁, 实际=%d", len(sim.WeldPatches))
	}

	thicknessIncreased := false
	for i := 0; i < sim.Grid.Nx && !thicknessIncreased; i++ {
		for j := 0; j < sim.Grid.Ny && !thicknessIncreased; j++ {
			for k := 0; k < sim.Grid.Nz; k++ {
				node := sim.Grid.Nodes[i][j][k]
				if node.IsActive && node.Thickness > node.OriginalThickness {
					thicknessIncreased = true
					break
				}
			}
		}
	}
	if !thicknessIncreased {
		t.Error("焊补物理模型: 应增厚钟壁")
	}

	stressIncreased := false
	for i := 0; i < sim.Grid.Nx; i++ {
		for j := 0; j < sim.Grid.Ny; j++ {
			for k := 0; k < sim.Grid.Nz; k++ {
				node := sim.Grid.Nodes[i][j][k]
				if node.IsActive && node.Stress > 1.0 {
					stressIncreased = true
					break
				}
			}
			if stressIncreased {
				break
			}
		}
		if stressIncreased {
			break
		}
	}
	if !stressIncreased {
		t.Error("焊补物理模型: 应增加节点应力(WeldingStressFactor=1.15)")
	}
}

func TestCrossEra_EvolutionComplexity(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	currentFreq := sim.CalculateEigenfrequencies()[0]
	results := sim.CompareTuningProcesses(currentFreq, currentFreq+10.0)

	var grindingComplexity, inlayComplexity, weldingComplexity int
	for _, r := range results {
		switch r.ProcessType {
		case ProcessGrinding:
			grindingComplexity = r.Complexity
		case ProcessCastingInlay:
			inlayComplexity = r.Complexity
		case ProcessWeldingRepair:
			weldingComplexity = r.Complexity
		}
	}

	if grindingComplexity >= inlayComplexity {
		t.Errorf("技术演进: 磨锉复杂度(%d)应低于铸镶(%d)", grindingComplexity, inlayComplexity)
	}
	if inlayComplexity >= weldingComplexity {
		t.Errorf("技术演进: 铸镶复杂度(%d)应低于焊补(%d)", inlayComplexity, weldingComplexity)
	}
}

func TestCrossEra_EvolutionReversibility(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	currentFreq := sim.CalculateEigenfrequencies()[0]
	results := sim.CompareTuningProcesses(currentFreq, currentFreq+10.0)

	for _, r := range results {
		switch r.ProcessType {
		case ProcessGrinding:
			if r.Reversibility {
				t.Error("技术演进: 磨锉(最古老工艺)应为不可逆")
			}
		case ProcessCastingInlay:
			if !r.Reversibility {
				t.Error("技术演进: 铸镶应为可逆(铅块可移除)")
			}
		case ProcessWeldingRepair:
			if !r.Reversibility {
				t.Error("技术演进: 焊补应为可逆(可磨去焊补)")
			}
		}
	}
}

func TestCrossEra_InlayDampingFactor(t *testing.T) {
	if InlayDampingFactor != 0.88 {
		t.Errorf("InlayDampingFactor应为0.88, 实际=%.2f", InlayDampingFactor)
	}
}

func TestCrossEra_WeldingStressFactor(t *testing.T) {
	if WeldingStressFactor != 1.15 {
		t.Errorf("WeldingStressFactor应为1.15, 实际=%.2f", WeldingStressFactor)
	}
}

func TestCrossEra_HarmonicityPenalty_InlayVsWelding(t *testing.T) {
	if InlayHarmonicityPenalty >= 0.95 {
		t.Errorf("铸镶和谐度惩罚(%.2f)应重于焊补(0.95)", InlayHarmonicityPenalty)
	}
}

func TestCalculateEigenfrequencies_Returns8(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	freqs := sim.CalculateEigenfrequencies()

	if len(freqs) != 8 {
		t.Errorf("应返回8个特征频率, 实际=%d", len(freqs))
	}
}

func TestCalculateEigenfrequencies_FundamentalPositive(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	freqs := sim.CalculateEigenfrequencies()

	if freqs[0] <= 0 {
		t.Errorf("基频应为正值, 实际=%.4f", freqs[0])
	}
}

func TestCalculateEigenfrequencies_HarmonicOrdering(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	freqs := sim.CalculateEigenfrequencies()

	for i := 1; i < len(freqs); i++ {
		if freqs[i] <= freqs[i-1] {
			t.Errorf("特征频率应递增: freq[%d]=%.2f <= freq[%d]=%.2f", i-1, freqs[i-1], i, freqs[i])
		}
	}
}

func TestCalculateEigenfrequencies_Ratios(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	freqs := sim.CalculateEigenfrequencies()

	idealRatios := []float64{1.0, 2.0, 3.0, 4.16, 5.42, 6.78, 8.15, 9.63}

	for i := 1; i < len(freqs); i++ {
		actualRatio := freqs[i] / freqs[0]
		deviation := math.Abs(actualRatio-idealRatios[i]) / idealRatios[i]
		if deviation > 0.05 {
			t.Errorf("第%d阶频率比偏差过大: 实际=%.4f, 理想=%.2f, 偏差=%.2f%%",
				i+1, actualRatio, idealRatios[i], deviation*100)
		}
	}
}

func TestCalculateHarmonicity_RangeBounds(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	harmonicity := sim.CalculateHarmonicity()

	if harmonicity < 0 || harmonicity > 1 {
		t.Errorf("和谐度应在[0,1]范围内, 实际=%.4f", harmonicity)
	}
}

func TestCalculateHarmonicity_NoPenaltyInitially(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	if len(sim.InlayMasses) != 0 {
		t.Error("初始状态不应有镶块记录")
	}
	if len(sim.WeldPatches) != 0 {
		t.Error("初始状态不应有焊补记录")
	}

	harmonicity := sim.CalculateHarmonicity()
	if harmonicity <= 0 {
		t.Errorf("无惩罚时和谐度应大于0, 实际=%.4f", harmonicity)
	}
}

func TestReset_ClearsAllState(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessGrinding, pos, 0.3)
	sim.ApplyTuningProcess(ProcessCastingInlay, pos, 0.3)
	sim.ApplyTuningProcess(ProcessWeldingRepair, pos, 0.3)

	if len(sim.ProcessHistory) == 0 {
		t.Fatal("应用工艺后应有历史记录")
	}

	sim.Reset()

	if len(sim.ProcessHistory) != 0 {
		t.Errorf("Reset后工艺历史应为空, 实际=%d", len(sim.ProcessHistory))
	}
	if len(sim.InlayMasses) != 0 {
		t.Errorf("Reset后镶块记录应为空, 实际=%d", len(sim.InlayMasses))
	}
	if len(sim.WeldPatches) != 0 {
		t.Errorf("Reset后焊补记录应为空, 实际=%d", len(sim.WeldPatches))
	}
	if len(sim.GrindingHistory) != 0 {
		t.Errorf("Reset后磨锉历史应为空, 实际=%d", len(sim.GrindingHistory))
	}
	if sim.LastQuality != nil {
		t.Error("Reset后LastQuality应为nil")
	}
}

func TestReset_RegeneratesGrid(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	firstGrid := sim.Grid
	firstGeneration := sim.Grid.Generation

	sim.Reset()

	if sim.Grid == nil {
		t.Fatal("Reset后应重新生成网格")
	}

	if sim.Grid.Generation <= firstGeneration {
		t.Errorf("Reset后网格代数应递增: before=%d, after=%d", firstGeneration, sim.Grid.Generation)
	}

	if sim.Grid == firstGrid {
		t.Error("Reset后应生成新的网格对象")
	}
}

func TestReset_FrequencyRestored(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	initialFreqs := sim.CalculateEigenfrequencies()
	initialFund := initialFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessGrinding, pos, 0.5)

	sim.Reset()

	resetFreqs := sim.CalculateEigenfrequencies()
	resetFund := resetFreqs[0]

	freqDiff := math.Abs(resetFund - initialFund)
	relativeDiff := freqDiff / initialFund

	if relativeDiff > 0.02 {
		t.Errorf("Reset后基频应恢复到初始值(±2%%): initial=%.2f, reset=%.2f, 相对偏差=%.2f%%",
			initialFund, resetFund, relativeDiff*100)
	}
}

func TestVirtualTuning_GrindingFrequencyDirection(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFund := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessGrinding, pos, 0.3)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFund := afterFreqs[0]

	if afterFund <= beforeFund {
		t.Errorf("虚拟调音: 磨锉应使频率升高, before=%.2f, after=%.2f", beforeFund, afterFund)
	}
}

func TestVirtualTuning_CastingInlayFrequencyDirection(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFund := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessCastingInlay, pos, 0.3)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFund := afterFreqs[0]

	if afterFund >= beforeFund {
		t.Errorf("虚拟调音: 铸镶应使频率降低, before=%.2f, after=%.2f", beforeFund, afterFund)
	}
}

func TestVirtualTuning_WeldingRepairFrequencyDirection(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFund := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessWeldingRepair, pos, 0.3)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFund := afterFreqs[0]

	if afterFund >= beforeFund {
		t.Errorf("虚拟调音: 焊补应使频率降低, before=%.2f, after=%.2f", beforeFund, afterFund)
	}
}

func TestApplyTuningProcess_ZeroDepth(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFund := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessGrinding, pos, 0.0)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFund := afterFreqs[0]

	relativeDiff := math.Abs(afterFund-beforeFund) / beforeFund
	if relativeDiff > 0.02 {
		t.Errorf("零深度磨锉应不改变频率(±2%%): before=%.2f, after=%.2f, 偏差=%.2f%%",
			beforeFund, afterFund, relativeDiff*100)
	}
}

func TestApplyTuningProcess_SmallDepth(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}

	sim.ApplyTuningProcess(ProcessGrinding, pos, 0.01)

	if len(sim.ProcessHistory) != 1 {
		t.Errorf("微小深度应仍然记录工艺历史, 实际=%d", len(sim.ProcessHistory))
	}
}

func TestCalculateEigenfrequencies_ThinBell(t *testing.T) {
	bell := &models.Bell{
		ID: 2, Name: "ThinBell", MassKg: 1.0, HeightCm: 20.0,
		DiameterCm: 15.0, ThicknessMm: 3.0, TargetFrequency: 440.0,
		ToleranceCents: 10.0, MaxGrindingDepthMm: 1.0,
	}
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	freqs := sim.CalculateEigenfrequencies()

	if len(freqs) != 8 {
		t.Errorf("薄壁编钟应返回8个频率, 实际=%d", len(freqs))
	}

	for i, f := range freqs {
		if f <= 0 {
			t.Errorf("第%d阶频率应为正值, 实际=%.4f", i, f)
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			t.Errorf("第%d阶频率不应为NaN或Inf", i)
		}
	}
}

func TestCalculateHarmonicity_ThinBell(t *testing.T) {
	bell := &models.Bell{
		ID: 3, Name: "ThinBell", MassKg: 1.0, HeightCm: 20.0,
		DiameterCm: 15.0, ThicknessMm: 3.0, TargetFrequency: 440.0,
		ToleranceCents: 10.0, MaxGrindingDepthMm: 1.0,
	}
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	harmonicity := sim.CalculateHarmonicity()

	if harmonicity < 0 || harmonicity > 1 {
		t.Errorf("薄壁编钟和谐度应在[0,1]范围内, 实际=%.4f", harmonicity)
	}
}

func TestApplyTuningProcess_ExcessiveDepth(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess(ProcessGrinding, pos, 10.0)

	quality := sim.EvaluateGridQuality()
	if quality == nil {
		t.Error("过度磨锉后仍应能评估网格质量")
	}
}

func TestCompareTuningProcesses_SameFreq(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	currentFreq := sim.CalculateEigenfrequencies()[0]

	results := sim.CompareTuningProcesses(currentFreq, currentFreq)

	if len(results) != 3 {
		t.Errorf("目标频率等于当前频率时仍应返回3个结果, 实际=%d", len(results))
	}

	for _, r := range results {
		if math.IsNaN(r.OverallScore) || math.IsInf(r.OverallScore, 0) {
			t.Errorf("工艺%s的OverallScore不应为NaN或Inf", r.ProcessType)
		}
	}
}

func TestCalculateProcessSensitivity_InvalidProcess(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sensitivity := sim.CalculateProcessSensitivity("invalid", pos)

	if math.IsNaN(sensitivity) || math.IsInf(sensitivity, 0) {
		t.Errorf("无效工艺类型的灵敏度不应为NaN或Inf, 实际=%.4f", sensitivity)
	}
}

func TestCalculateProcessSensitivity_PreservesGrid(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFund := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.CalculateProcessSensitivity(ProcessGrinding, pos)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFund := afterFreqs[0]

	relativeDiff := math.Abs(afterFund-beforeFund) / beforeFund
	if relativeDiff > 0.02 {
		t.Errorf("CalculateProcessSensitivity应恢复网格状态: before=%.2f, after=%.2f, 偏差=%.2f%%",
			beforeFund, afterFund, relativeDiff*100)
	}
}

func TestCompareTuningProcesses_PreservesGrid(t *testing.T) {
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
		t.Errorf("CompareTuningProcesses应恢复网格状态: before=%.2f, after=%.2f, 偏差=%.2f%%",
			beforeFund, afterFund, relativeDiff*100)
	}
}

func TestApplyTuningProcess_MultipleProcesses(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}

	sim.ApplyTuningProcess(ProcessGrinding, pos, 0.2)
	sim.ApplyTuningProcess(ProcessCastingInlay, pos, 0.3)
	sim.ApplyTuningProcess(ProcessWeldingRepair, pos, 0.2)

	if len(sim.ProcessHistory) != 3 {
		t.Errorf("应用3种工艺后应有3条记录, 实际=%d", len(sim.ProcessHistory))
	}

	if sim.ProcessHistory[0].ProcessType != ProcessGrinding {
		t.Errorf("第1条记录应为grinding, 实际=%s", sim.ProcessHistory[0].ProcessType)
	}
	if sim.ProcessHistory[1].ProcessType != ProcessCastingInlay {
		t.Errorf("第2条记录应为casting_inlay, 实际=%s", sim.ProcessHistory[1].ProcessType)
	}
	if sim.ProcessHistory[2].ProcessType != ProcessWeldingRepair {
		t.Errorf("第3条记录应为welding_repair, 实际=%s", sim.ProcessHistory[2].ProcessType)
	}

	if len(sim.InlayMasses) != 1 {
		t.Errorf("应有1个镶块记录, 实际=%d", len(sim.InlayMasses))
	}
	if len(sim.WeldPatches) != 1 {
		t.Errorf("应有1个焊补记录, 实际=%d", len(sim.WeldPatches))
	}
}

func TestGenerateGrid_CreatesActiveNodes(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)

	grid := sim.GenerateGrid()

	if grid == nil {
		t.Fatal("GenerateGrid应返回非nil网格")
	}

	activeCount := 0
	for i := 0; i < grid.Nx; i++ {
		for j := 0; j < grid.Ny; j++ {
			for k := 0; k < grid.Nz; k++ {
				if grid.Nodes[i][j][k].IsActive {
					activeCount++
				}
			}
		}
	}

	if activeCount == 0 {
		t.Error("生成的网格应包含活跃节点")
	}
}

func TestEvaluateGridQuality_InitialGrid(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	report := sim.EvaluateGridQuality()

	if report == nil {
		t.Fatal("EvaluateGridQuality应返回非nil报告")
	}

	if report.ActiveNodeCount == 0 {
		t.Error("初始网格应有活跃节点")
	}

	if report.AverageThickness <= 0 {
		t.Errorf("平均厚度应为正值, 实际=%.4f", report.AverageThickness)
	}
}

func TestEvaluateGridQuality_NilGrid(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)

	report := sim.EvaluateGridQuality()

	if report == nil {
		t.Fatal("nil网格时仍应返回报告")
	}

	if !report.ShouldRebuild {
		t.Error("nil网格时ShouldRebuild应为true")
	}
}

func TestCompareTuningProcesses_OverallScoreRange(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	currentFreq := sim.CalculateEigenfrequencies()[0]
	results := sim.CompareTuningProcesses(currentFreq, currentFreq+15.0)

	for _, r := range results {
		if r.OverallScore < 0 || r.OverallScore > 1 {
			t.Errorf("工艺%s的OverallScore应在[0,1]范围内, 实际=%.4f", r.ProcessType, r.OverallScore)
		}
	}
}

func TestCompareTuningProcesses_CostScoreOrder(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	currentFreq := sim.CalculateEigenfrequencies()[0]
	results := sim.CompareTuningProcesses(currentFreq, currentFreq+10.0)

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

func TestApplyGrinding_RecordsHistory(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyGrinding(pos, 0.3)

	if len(sim.GrindingHistory) != 1 {
		t.Errorf("ApplyGrinding应记录1条磨锉历史, 实际=%d", len(sim.GrindingHistory))
	}

	if sim.GrindingHistory[0].DepthMm != 0.3 {
		t.Errorf("记录深度应为0.3, 实际=%.2f", sim.GrindingHistory[0].DepthMm)
	}
}

func TestNewFEMSimulator_InitializesEmpty(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)

	if sim.Bell != bell {
		t.Error("模拟器应持有传入的编钟对象")
	}
	if sim.Grid != nil {
		t.Error("新模拟器Grid应为nil")
	}
	if len(sim.GrindingHistory) != 0 {
		t.Error("新模拟器磨锉历史应为空")
	}
	if len(sim.ProcessHistory) != 0 {
		t.Error("新模拟器工艺历史应为空")
	}
	if len(sim.InlayMasses) != 0 {
		t.Error("新模拟器镶块记录应为空")
	}
	if len(sim.WeldPatches) != 0 {
		t.Error("新模拟器焊补记录应为空")
	}
}

func TestWeldingRepair_IncreasesStress(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}

	var beforeStress float64
	for i := 0; i < sim.Grid.Nx; i++ {
		for j := 0; j < sim.Grid.Ny; j++ {
			for k := 0; k < sim.Grid.Nz; k++ {
				node := sim.Grid.Nodes[i][j][k]
				if node.IsActive {
					beforeStress += node.Stress
				}
			}
		}
	}

	sim.ApplyTuningProcess(ProcessWeldingRepair, pos, 0.5)

	var afterStress float64
	for i := 0; i < sim.Grid.Nx; i++ {
		for j := 0; j < sim.Grid.Ny; j++ {
			for k := 0; k < sim.Grid.Nz; k++ {
				node := sim.Grid.Nodes[i][j][k]
				if node.IsActive {
					afterStress += node.Stress
				}
			}
		}
	}

	if afterStress <= beforeStress {
		t.Errorf("焊补应增加节点应力: before=%.4f, after=%.4f", beforeStress, afterStress)
	}
}
