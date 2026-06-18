package vr_tuning

import (
	"testing"

	"bianzhong-acoustic-system/models"
	"bianzhong-acoustic-system/simulation"
)

func testBell() *models.Bell {
	return &models.Bell{
		ID:                 1,
		Name:               "test-bell",
		MassKg:             2.5,
		HeightCm:           30,
		DiameterCm:         20,
		ThicknessMm:        8.0,
		TargetFrequency:    440.0,
		ToleranceCents:     10.0,
		MaxGrindingDepthMm: 2.0,
	}
}

func TestNewVRTuningSession_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-session-1", bell)

	if session == nil {
		t.Fatal("VRTuningSession should not be nil")
	}
	if session.GetSessionID() != "test-session-1" {
		t.Errorf("SessionID应为test-session-1, 实际=%s", session.GetSessionID())
	}
	if session.GetCurrentFreq() != 440.0 {
		t.Errorf("初始频率应为440Hz, 实际=%.2f", session.GetCurrentFreq())
	}
	if session.GetTolerance() != 10.0 {
		t.Errorf("容差应为10音分, 实际=%.2f", session.GetTolerance())
	}
}

func TestVRTuningSession_GrindAsync_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-grind", bell)

	pos := models.GrindingPosition{X: 0, Y: 0, Z: 0}
	beforeFreq := session.GetCurrentFreq()

	resultCh := session.GrindAsync(pos, 0.1)
	result := <-resultCh

	if result.Error != nil {
		t.Fatalf("磨锉不应出错: %v", result.Error)
	}

	grind, ok := result.Result.(models.VirtualGrind)
	if !ok {
		t.Fatal("结果类型应为VirtualGrind")
	}

	if grind.BeforeFreq != beforeFreq {
		t.Errorf("磨前频率不匹配: 预期=%.2f, 实际=%.2f", beforeFreq, grind.BeforeFreq)
	}
	if grind.ProcessType != simulation.ProcessGrinding {
		t.Errorf("工艺类型应为grinding, 实际=%s", grind.ProcessType)
	}
}

func TestVRTuningSession_ApplyProcessAsync_Grinding_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-apply", bell)

	pos := models.GrindingPosition{X: 0, Y: 0, Z: 0}
	resultCh := session.ApplyProcessAsync(simulation.ProcessGrinding, pos, 0.1)
	result := <-resultCh

	if result.Error != nil {
		t.Fatalf("应用工艺不应出错: %v", result.Error)
	}

	grind, ok := result.Result.(models.VirtualGrind)
	if !ok {
		t.Fatal("结果类型应为VirtualGrind")
	}
	if grind.ProcessType != simulation.ProcessGrinding {
		t.Error("工艺类型应为grinding")
	}
}

func TestVRTuningSession_GetEigenfrequenciesAsync_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-freq", bell)

	resultCh := session.GetEigenfrequenciesAsync()
	result := <-resultCh

	freqs, ok := result.Result.([]float64)
	if !ok {
		t.Fatal("结果类型应为[]float64")
	}
	if len(freqs) == 0 {
		t.Error("特征频率列表不应为空")
	}
	for _, f := range freqs {
		if f <= 0 {
			t.Errorf("频率应为正, 实际=%.2f", f)
		}
	}
}

func TestVRTuningSession_GetDeviationCents_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-deviation", bell)

	dev := session.GetDeviationCents()
	if dev != 0 {
		t.Logf("初始偏差: %.4f 音分", dev)
	}
}

func TestVRTuningSession_IsWithinTolerance_Initial_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-tolerance", bell)

	if !session.IsWithinTolerance() {
		t.Error("初始状态应在容差范围内")
	}
}

func TestVRTuningSession_Reset_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-reset", bell)

	pos := models.GrindingPosition{X: 0, Y: 0, Z: 0}
	<-session.GrindAsync(pos, 0.2)

	if session.GetTotalDepth() <= 0 {
		t.Error("磨锉后总深度应大于0")
	}
	if len(session.GetHistory()) != 1 {
		t.Errorf("历史记录应有1条, 实际=%d", len(session.GetHistory()))
	}

	session.Reset()

	if session.GetTotalDepth() != 0 {
		t.Errorf("重置后总深度应为0, 实际=%.4f", session.GetTotalDepth())
	}
	if len(session.GetHistory()) != 0 {
		t.Errorf("重置后历史记录应为空, 实际=%d", len(session.GetHistory()))
	}
}

func TestVRTuningSession_GetHistory_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-history", bell)

	pos := models.GrindingPosition{X: 0, Y: 0, Z: 0}
	<-session.GrindAsync(pos, 0.1)
	<-session.GrindAsync(pos, 0.2)

	history := session.GetHistory()
	if len(history) != 2 {
		t.Errorf("历史记录应有2条, 实际=%d", len(history))
	}

	history[0].DepthMm = 999
	history2 := session.GetHistory()
	if history2[0].DepthMm == 999 {
		t.Error("GetHistory应返回副本, 不应修改内部状态")
	}
}

func TestVRTuningSession_GetTotalDepth_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-depth", bell)

	pos := models.GrindingPosition{X: 0, Y: 0, Z: 0}
	<-session.GrindAsync(pos, 0.1)
	<-session.GrindAsync(pos, 0.3)

	total := session.GetTotalDepth()
	expected := 0.4
	if total < expected-0.001 || total > expected+0.001 {
		t.Errorf("总深度应为%.2f, 实际=%.4f", expected, total)
	}
}

func TestVRTuningSession_SetTolerance_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-tol-set", bell)

	session.SetTolerance(25.0)
	if session.GetTolerance() != 25.0 {
		t.Errorf("容差应设为25, 实际=%.2f", session.GetTolerance())
	}
}

func TestVRTuningSession_GetHarmonicity_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-harmonicity", bell)

	h := session.GetHarmonicity()
	if h < 0 || h > 1 {
		t.Errorf("和谐度应在0-1之间, 实际=%.4f", h)
	}
}

func TestVRTuningSession_GetAmplitudes_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-amp", bell)

	amps := session.GetAmplitudes()
	if len(amps) == 0 {
		t.Error("振幅列表不应为空")
	}
	for i, a := range amps {
		if a < 0 {
			t.Errorf("第%d个振幅应为非负, 实际=%.4f", i, a)
		}
	}
}

func TestVRTuningSession_GetDecayRates_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-decay", bell)

	decays := session.GetDecayRates()
	if len(decays) != 8 {
		t.Errorf("衰减率应有8个, 实际=%d", len(decays))
	}
}

func TestVRTuningSession_ToModel_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-model", bell)

	model := session.ToModel()
	if model.SessionID != "test-model" {
		t.Errorf("模型SessionID不匹配: 预期=test-model, 实际=%s", model.SessionID)
	}
	if model.BellID != 1 {
		t.Errorf("模型BellID不匹配: 预期=1, 实际=%d", model.BellID)
	}
	if model.TargetFreq != 440.0 {
		t.Errorf("模型目标频率不匹配: 预期=440, 实际=%.2f", model.TargetFreq)
	}
}

func TestVRTuningSession_GetBell_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-bell-get", bell)

	b := session.GetBell()
	if b.ID != 1 {
		t.Errorf("Bell ID应为1, 实际=%d", b.ID)
	}
}

func TestVRTuningSession_GetTargetFreq_Normal(t *testing.T) {
	bell := testBell()
	session := NewVRTuningSession("test-target", bell)

	if session.GetTargetFreq() != 440.0 {
		t.Errorf("目标频率应为440, 实际=%.2f", session.GetTargetFreq())
	}
}

func TestAsyncResult_Structure_Normal(t *testing.T) {
	result := AsyncResult{
		SessionID: "test",
		Result:    "hello",
		Error:     nil,
	}

	if result.SessionID != "test" {
		t.Error("SessionID不匹配")
	}
	if result.Result != "hello" {
		t.Error("Result不匹配")
	}
	if result.Error != nil {
		t.Error("Error应为nil")
	}
}
