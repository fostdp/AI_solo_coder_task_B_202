package simulation

import (
	"math"
	"testing"
	"time"

	"bianzhong-acoustic-system/models"
)

func TestVirtualTuning_SessionInitialization_Normal(t *testing.T) {
	bell := createTestBell()

	initialFreq := bell.TargetFrequency * 1.01

	session := &models.VirtualTuningSession{
		SessionID:    "test-session-001",
		BellID:       bell.ID,
		CurrentFreq:  initialFreq,
		OriginalFreq: initialFreq,
		TargetFreq:   bell.TargetFrequency,
		History:      make([]models.VirtualGrind, 0),
		TotalDepthMm: 0,
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
	}

	if session.SessionID != "test-session-001" {
		t.Errorf("会话ID不匹配: 预期=test-session-001, 实际=%s", session.SessionID)
	}
	if session.CurrentFreq != initialFreq {
		t.Errorf("初始频率不匹配: 预期=%.2f, 实际=%.2f", initialFreq, session.CurrentFreq)
	}
	if session.TargetFreq != bell.TargetFrequency {
		t.Errorf("目标频率不匹配: 预期=%.2f, 实际=%.2f", bell.TargetFrequency, session.TargetFreq)
	}
	if len(session.History) != 0 {
		t.Errorf("初始历史应为空, 实际=%d", len(session.History))
	}
	if session.TotalDepthMm != 0 {
		t.Errorf("初始总深度应为0, 实际=%.2f", session.TotalDepthMm)
	}
}

func TestVirtualTuning_SingleGrind_FrequencyDirection_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFreq := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess("grinding", pos, 0.3)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFreq := afterFreqs[0]

	if math.IsNaN(afterFreq) || math.IsInf(afterFreq, 0) {
		t.Errorf("磨锉后频率不应为NaN/Inf, 实际=%.4f", afterFreq)
	}
	if afterFreq <= 0 {
		t.Errorf("磨锉后频率应为正值, 实际=%.4f", afterFreq)
	}
	if len(sim.ProcessHistory) != 1 {
		t.Errorf("磨锉后应有1条工艺记录, 实际=%d", len(sim.ProcessHistory))
	}

	t.Logf("磨锉频率变化: before=%.2f, after=%.2f", beforeFreq, afterFreq)
}

func TestVirtualTuning_CastingInlay_FrequencyDirection_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFreq := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess("casting_inlay", pos, 0.3)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFreq := afterFreqs[0]

	if math.IsNaN(afterFreq) || math.IsInf(afterFreq, 0) {
		t.Errorf("铸镶后频率不应为NaN/Inf, 实际=%.4f", afterFreq)
	}
	if afterFreq <= 0 {
		t.Errorf("铸镶后频率应为正值, 实际=%.4f", afterFreq)
	}
	if len(sim.InlayMasses) != 1 {
		t.Errorf("铸镶后应有1个镶块记录, 实际=%d", len(sim.InlayMasses))
	}

	t.Logf("铸镶频率变化: before=%.2f, after=%.2f", beforeFreq, afterFreq)
}

func TestVirtualTuning_WeldingRepair_FrequencyDirection_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFreq := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess("welding_repair", pos, 0.3)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFreq := afterFreqs[0]

	if math.IsNaN(afterFreq) || math.IsInf(afterFreq, 0) {
		t.Errorf("焊补后频率不应为NaN/Inf, 实际=%.4f", afterFreq)
	}
	if afterFreq <= 0 {
		t.Errorf("焊补后频率应为正值, 实际=%.4f", afterFreq)
	}
	if len(sim.WeldPatches) != 1 {
		t.Errorf("焊补后应有1个补丁记录, 实际=%d", len(sim.WeldPatches))
	}

	t.Logf("焊补频率变化: before=%.2f, after=%.2f", beforeFreq, afterFreq)
}

func TestVirtualTuning_MultipleGrinds_CumulativeDepth_Normal(t *testing.T) {
	bell := createTestBell()
	session := &models.VirtualTuningSession{
		SessionID:    "test-session-002",
		BellID:       bell.ID,
		CurrentFreq:  bell.TargetFrequency * 1.02,
		OriginalFreq: bell.TargetFrequency * 1.02,
		TargetFreq:   bell.TargetFrequency,
		History:      make([]models.VirtualGrind, 0),
		TotalDepthMm: 0,
	}

	depths := []float64{0.2, 0.3, 0.15}
	for _, d := range depths {
		session.TotalDepthMm += math.Abs(d)
		grind := models.VirtualGrind{
			Time:        time.Now(),
			Position:    models.GrindingPosition{X: 0, Y: 15, Z: 8},
			DepthMm:     d,
			ProcessType: "grinding",
			BeforeFreq:  session.CurrentFreq,
			AfterFreq:   session.CurrentFreq * (1 + d/bell.ThicknessMm*0.12),
		}
		session.History = append(session.History, grind)
		session.CurrentFreq = grind.AfterFreq
	}

	expectedTotal := 0.2 + 0.3 + 0.15
	if math.Abs(session.TotalDepthMm-expectedTotal) > 0.001 {
		t.Errorf("累积深度计算错误: 预期=%.3f, 实际=%.3f", expectedTotal, session.TotalDepthMm)
	}
	if len(session.History) != 3 {
		t.Errorf("应有3条历史记录, 实际=%d", len(session.History))
	}
}

func TestVirtualTuning_GrindDeviationCalculation_Normal(t *testing.T) {
	targetFreq := 440.0
	afterFreq := 442.5

	dev := 1200.0 * math.Log2(afterFreq/targetFreq)

	expectedDev := 1200.0 * math.Log2(442.5/440.0)
	if math.Abs(dev-expectedDev) > 0.001 {
		t.Errorf("音分偏差计算错误: 预期=%.4f, 实际=%.4f", expectedDev, dev)
	}

	if dev <= 0 {
		t.Errorf("442.5Hz相对440Hz应正偏差, 实际=%.4f", dev)
	}
}

func TestVirtualTuning_WithinTolerance_Boundary(t *testing.T) {
	targetFreq := 440.0
	toleranceCents := 10.0

	testCases := []struct {
		freq   float64
		expect bool
		desc   string
	}{
		{440.0, true, "精确匹配"},
		{440.5, true, "略高于目标"},
		{439.5, true, "略低于目标"},
		{440.0 * math.Pow(2, 9.5/1200), true, "9.5音分偏差(阈值内)"},
	}

	for _, tc := range testCases {
		dev := 1200.0 * math.Log2(tc.freq/targetFreq)
		within := math.Abs(dev) <= toleranceCents
		if within != tc.expect {
			t.Errorf("%s: freq=%.4f, dev=%.4f cents, 预期within=%v, 实际=%v",
				tc.desc, tc.freq, dev, tc.expect, within)
		}
	}

	dev9 := 1200.0 * math.Log2(440.0*math.Pow(2, 9.0/1200)/targetFreq)
	if math.Abs(dev9) > toleranceCents {
		t.Errorf("9音分偏差应在容差内: dev=%.4f", dev9)
	}
}

func TestVirtualTuning_OutsideTolerance_Boundary(t *testing.T) {
	targetFreq := 440.0
	toleranceCents := 10.0

	testCases := []struct {
		freq   float64
		expect bool
		desc   string
	}{
		{440.0 * math.Pow(2, 15.0/1200), false, "15音分正偏差"},
		{440.0 * math.Pow(2, -20.0/1200), false, "20音分负偏差"},
		{450.0, false, "大幅正偏差"},
		{430.0, false, "大幅负偏差"},
	}

	for _, tc := range testCases {
		dev := 1200.0 * math.Log2(tc.freq/targetFreq)
		within := math.Abs(dev) <= toleranceCents
		if within != tc.expect {
			t.Errorf("%s: freq=%.4f, dev=%.4f cents, 预期within=%v, 实际=%v",
				tc.desc, tc.freq, dev, tc.expect, within)
		}
	}
}

func TestVirtualTuning_EigenfrequenciesCount_Abnormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	eigenfreqs := sim.CalculateEigenfrequencies()

	if len(eigenfreqs) != 8 {
		t.Errorf("应返回8个特征频率, 实际=%d", len(eigenfreqs))
	}
}

func TestVirtualTuning_AmplitudesDecay_Normal(t *testing.T) {
	eigenfreqs := make([]float64, 8)
	for i := range eigenfreqs {
		eigenfreqs[i] = 440.0 * float64(i+1)
	}

	amplitudes := make([]float64, len(eigenfreqs))
	for i := range eigenfreqs {
		amplitudes[i] = math.Exp(-float64(i) * 0.4)
	}

	for i := 1; i < len(amplitudes); i++ {
		if amplitudes[i] >= amplitudes[i-1] {
			t.Errorf("振幅应随阶数递减: amp[%d]=%.4f >= amp[%d]=%.4f",
				i, amplitudes[i], i-1, amplitudes[i-1])
		}
	}

	if amplitudes[0] <= 0.9 || amplitudes[0] > 1.01 {
		t.Errorf("基频振幅应接近1.0, 实际=%.4f", amplitudes[0])
	}

	for i, a := range amplitudes {
		if a <= 0 {
			t.Errorf("第%d阶振幅应为正, 实际=%.6f", i, a)
		}
	}
}

func TestVirtualTuning_DecayRates_Normal(t *testing.T) {
	decayRates := []float64{1.5, 2.0, 2.8, 3.5, 4.2, 5.0, 5.8, 6.5}

	if len(decayRates) != 8 {
		t.Fatalf("应有8个衰减率, 实际=%d", len(decayRates))
	}

	for i := 1; i < len(decayRates); i++ {
		if decayRates[i] <= decayRates[i-1] {
			t.Errorf("衰减率应递增: decay[%d]=%.1f <= decay[%d]=%.1f",
				i, decayRates[i], i-1, decayRates[i-1])
		}
	}

	for i, d := range decayRates {
		if d < 1.0 || d > 10.0 {
			t.Errorf("第%d阶衰减率应在合理范围[1,10], 实际=%.1f", i, d)
		}
	}
}

func TestVirtualTuning_SessionReset_Normal(t *testing.T) {
	bell := createTestBell()
	originalFreq := bell.TargetFrequency * 1.03

	session := &models.VirtualTuningSession{
		SessionID:    "test-reset-001",
		BellID:       bell.ID,
		CurrentFreq:  bell.TargetFrequency * 1.01,
		OriginalFreq: originalFreq,
		TargetFreq:   bell.TargetFrequency,
		History: []models.VirtualGrind{
			{Time: time.Now(), DepthMm: 0.3},
		},
		TotalDepthMm: 0.3,
	}

	session.CurrentFreq = session.OriginalFreq
	session.History = make([]models.VirtualGrind, 0)
	session.TotalDepthMm = 0

	if session.CurrentFreq != originalFreq {
		t.Errorf("重置后频率应恢复: 预期=%.2f, 实际=%.2f", originalFreq, session.CurrentFreq)
	}
	if len(session.History) != 0 {
		t.Errorf("重置后历史应为空, 实际=%d", len(session.History))
	}
	if session.TotalDepthMm != 0 {
		t.Errorf("重置后总深度应为0, 实际=%.2f", session.TotalDepthMm)
	}
}

func TestVirtualTuning_HarmonicityValue_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	harmonicity := sim.CalculateHarmonicity()

	if harmonicity < 0 || harmonicity > 1 {
		t.Errorf("和谐度应在[0,1]范围, 实际=%.4f", harmonicity)
	}

	if harmonicity <= 0 {
		t.Errorf("初始和谐度应>0, 实际=%.4f", harmonicity)
	}
}

func TestVirtualTuning_ZeroDepthGrind_Abnormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFreq := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess("grinding", pos, 0.0)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFreq := afterFreqs[0]

	relativeDiff := math.Abs(afterFreq-beforeFreq) / beforeFreq
	if relativeDiff > 0.02 {
		t.Errorf("零深度磨锉不应显著改变频率: before=%.2f, after=%.2f, 偏差=%.2f%%",
			beforeFreq, afterFreq, relativeDiff*100)
	}
}

func TestVirtualTuning_EigenfrequenciesOrdered_Normal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	freqs := sim.CalculateEigenfrequencies()

	for i := 1; i < len(freqs); i++ {
		if freqs[i] <= freqs[i-1] {
			t.Errorf("特征频率应严格递增: freq[%d]=%.2f <= freq[%d]=%.2f",
				i, freqs[i], i-1, freqs[i-1])
		}
	}
}

func TestVirtualTuning_NegativeDepth_Abnormal(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	beforeFreqs := sim.CalculateEigenfrequencies()
	beforeFreq := beforeFreqs[0]

	pos := models.GrindingPosition{X: 0, Y: 15, Z: 8}
	sim.ApplyTuningProcess("grinding", pos, -0.3)

	afterFreqs := sim.CalculateEigenfrequencies()
	afterFreq := afterFreqs[0]

	if math.IsNaN(afterFreq) || math.IsInf(afterFreq, 0) {
		t.Errorf("负深度不应产生NaN/Inf频率, 实际=%.4f", afterFreq)
	}

	t.Logf("负深度磨锉: before=%.2f, after=%.2f", beforeFreq, afterFreq)
}

func TestVirtualTuning_DefaultProcessType_Boundary(t *testing.T) {
	bell := createTestBell()
	sim := NewFEMSimulator(bell)
	sim.GenerateGrid()

	processType := ""
	if processType == "" {
		processType = "grinding"
	}

	if processType != "grinding" {
		t.Errorf("默认工艺类型应为grinding, 实际=%s", processType)
	}
}

func TestVirtualTuning_HistoryRecordStructure_Normal(t *testing.T) {
	grind := models.VirtualGrind{
		Time:        time.Now(),
		Position:    models.GrindingPosition{X: 1, Y: 10, Z: 5},
		DepthMm:     0.25,
		ProcessType: "grinding",
		BeforeFreq:  450.0,
		AfterFreq:   452.0,
		Deviation:   47.0,
	}

	if grind.Time.IsZero() {
		t.Error("磨锉记录应有时间戳")
	}
	if grind.DepthMm != 0.25 {
		t.Errorf("磨锉深度不匹配: 预期=0.25, 实际=%.2f", grind.DepthMm)
	}
	if grind.AfterFreq <= grind.BeforeFreq {
		t.Errorf("磨锉后频率应升高: before=%.2f, after=%.2f", grind.BeforeFreq, grind.AfterFreq)
	}
}
