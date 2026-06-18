package era_comparator

import (
	"testing"

	"bianzhong-acoustic-system/models"
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

func TestNewEraComparator_Normal(t *testing.T) {
	bell := testBell()
	ec := NewEraComparator(bell)
	if ec == nil {
		t.Fatal("EraComparator should not be nil")
	}
}

func TestEraComparator_CompareEras_Normal(t *testing.T) {
	bell := testBell()
	ec := NewEraComparator(bell)

	metrics := ec.CompareEras(430, 440)

	if len(metrics) != 3 {
		t.Errorf("应有3个时代的指标, 实际=%d", len(metrics))
	}

	for _, m := range metrics {
		if m.EraName == "" {
			t.Error("时代名称不应为空")
		}
		if m.ComplexityScore < 0 || m.ComplexityScore > 1 {
			t.Errorf("复杂度评分应在0-1之间, 实际=%.4f", m.ComplexityScore)
		}
		if m.ReversibilityScore < 0 || m.ReversibilityScore > 1 {
			t.Errorf("可逆性评分应在0-1之间, 实际=%.4f", m.ReversibilityScore)
		}
		if m.DamageRiskScore < 0 || m.DamageRiskScore > 1 {
			t.Errorf("损伤风险评分应在0-1之间, 实际=%.4f", m.DamageRiskScore)
		}
		if m.EstimatedHours < 0 {
			t.Errorf("预估时间不应为负, 实际=%.2f", m.EstimatedHours)
		}
		if m.HistoricalEra == "" {
			t.Error("历史时期不应为空")
		}
	}
}

func TestEraComparator_CompareErasAsync_Normal(t *testing.T) {
	bell := testBell()
	ec := NewEraComparator(bell)

	resultCh := ec.CompareErasAsync(430, 440)
	metrics := <-resultCh

	if len(metrics) != 3 {
		t.Errorf("异步对比应有3个时代, 实际=%d", len(metrics))
	}
}

func TestEraComparator_GetEraDetail_Normal(t *testing.T) {
	bell := testBell()
	ec := NewEraComparator(bell)

	detail := ec.GetEraDetail("grinding", 430, 440)
	if detail == nil {
		t.Fatal("应找到grinding时代的详情")
	}
	if detail.EraName != "grinding" {
		t.Errorf("时代名称应为grinding, 实际=%s", detail.EraName)
	}
}

func TestEraComparator_GetEraDetail_NotFound_Boundary(t *testing.T) {
	bell := testBell()
	ec := NewEraComparator(bell)

	detail := ec.GetEraDetail("invalid_era", 430, 440)
	if detail != nil {
		t.Error("不存在的时代应返回nil")
	}
}

func TestEraComparator_EvolutionIndex_Normal(t *testing.T) {
	bell := testBell()
	ec := NewEraComparator(bell)

	idx := ec.EvolutionIndex()
	t.Logf("演进指数: %.2f%%", idx)
}

func TestEraComparator_TechnologyProgressScore_Normal(t *testing.T) {
	bell := testBell()
	ec := NewEraComparator(bell)

	scores := ec.TechnologyProgressScore()
	if len(scores) != 3 {
		t.Errorf("应有3个时代的技术评分, 实际=%d", len(scores))
	}

	for era, score := range scores {
		if score < 0 || score > 100 {
			t.Errorf("%s 的技术评分应在0-100之间, 实际=%.2f", era, score)
		}
	}
}

func TestEraComparator_Reset_Normal(t *testing.T) {
	bell := testBell()
	ec := NewEraComparator(bell)

	ec.Reset()

	metrics := ec.CompareEras(430, 440)
	if len(metrics) != 3 {
		t.Error("重置后应仍能正常对比")
	}
}

func TestEraMetrics_Structure_Normal(t *testing.T) {
	m := EraMetrics{
		EraName:            "test",
		ComplexityScore:    0.5,
		ReversibilityScore: 0.7,
		DamageRiskScore:    0.3,
		EstimatedHours:     2.5,
		HarmonicityImpact:  0.8,
		HistoricalEra:      "ancient",
	}

	if m.EraName != "test" {
		t.Error("EraName不匹配")
	}
	if m.HistoricalEra != "ancient" {
		t.Error("HistoricalEra不匹配")
	}
}

func TestEraNameConstants_Normal(t *testing.T) {
	if EraGrinding != "grinding" {
		t.Errorf("EraGrinding应为grinding, 实际=%s", EraGrinding)
	}
	if EraCastingInlay != "casting_inlay" {
		t.Errorf("EraCastingInlay应为casting_inlay, 实际=%s", EraCastingInlay)
	}
	if EraWeldingRepair != "welding_repair" {
		t.Errorf("EraWeldingRepair应为welding_repair, 实际=%s", EraWeldingRepair)
	}
}
