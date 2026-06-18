package technique_comparator

import (
	"math"
	"testing"

	"bianzhong-acoustic-system/models"
)

func TestNewTechniqueComparator_Normal(t *testing.T) {
	bell := &models.Bell{
		ID:          1,
		Name:         "test-bell",
		MassKg:       2.5,
		HeightCm:     30,
		DiameterCm:   20,
		ThicknessMm:  8.0,
		TargetFrequency: 440,
		ToleranceCents: 10,
	}

	tc := NewTechniqueComparator(bell)
	if tc == nil {
		t.Fatal("TechniqueComparator should not be nil")
	}
}

func TestTechniqueComparator_Compare_Normal(t *testing.T) {
	bell := &models.Bell{
		ID:              1,
		Name:            "test-bell",
		MassKg:          2.5,
		HeightCm:        30,
		DiameterCm:      20,
		ThicknessMm:     8.0,
		TargetFrequency: 440,
		ToleranceCents:  10,
	}

	tc := NewTechniqueComparator(bell)
	results := tc.Compare(430, 440)

	if len(results) != 3 {
		t.Errorf("应有3种工艺对比结果, 实际=%d", len(results))
	}

	for _, r := range results {
		if r.ProcessType == "" {
			t.Error("工艺类型不应为空")
		}
		if r.OverallScore < 0 || r.OverallScore > 1 {
			t.Errorf("工艺 %s 综合评分应在0-1之间, 实际=%.4f", r.ProcessType, r.OverallScore)
		}
	}
}

func TestTechniqueComparator_CompareAsync_Normal(t *testing.T) {
	bell := &models.Bell{
		ID:              1,
		Name:            "test-bell",
		MassKg:          2.5,
		HeightCm:        30,
		DiameterCm:      20,
		ThicknessMm:     8.0,
		TargetFrequency: 440,
		ToleranceCents:  10,
	}

	tc := NewTechniqueComparator(bell)
	resultCh := tc.CompareAsync(430, 440)

	results := <-resultCh

	if len(results) != 3 {
		t.Errorf("异步应有3种工艺, 实际=%d", len(results))
	}
}

func TestTechniqueComparator_BestProcess_Normal(t *testing.T) {
	bell := &models.Bell{
		ID:              1,
		Name:            "test-bell",
		MassKg:          2.5,
		HeightCm:        30,
		DiameterCm:      20,
		ThicknessMm:     8.0,
		TargetFrequency: 440,
		ToleranceCents:  10,
	}

	tc := NewTechniqueComparator(bell)
	best := tc.BestProcess(430, 440)

	if best == nil {
		t.Fatal("最佳工艺不应为nil")
	}
	if best.ProcessType == "" {
		t.Error("最佳工艺类型不应为空")
	}
}

func TestFindResult_Normal(t *testing.T) {
	bell := &models.Bell{
		ID:              1,
		Name:            "test-bell",
		MassKg:          2.5,
		HeightCm:        30,
		DiameterCm:      20,
		ThicknessMm:     8.0,
		TargetFrequency: 440,
		ToleranceCents:  10,
	}

	tc := NewTechniqueComparator(bell)
	results := tc.Compare(430, 440)

	found := FindResult(results, ProcessGrinding)
	if found == nil {
		t.Fatal("应找到grinding工艺")
	}
	if found.ProcessType != ProcessGrinding {
		t.Errorf("应为grinding, 实际=%s", found.ProcessType)
	}

	notFound := FindResult(results, "invalid")
	if notFound != nil {
		t.Error("无效工艺应返回nil")
	}
}

func TestAccuracyRank_Normal(t *testing.T) {
	results := []models.ProcessComparisonResult{
		{ProcessType: "a", DeviationCents: 10},
		{ProcessType: "b", DeviationCents: 5},
		{ProcessType: "c", DeviationCents: 8},
	}

	ranked := AccuracyRank(results)
	if len(ranked) != 3 {
		t.Fatal("排序后数量应保持3条")
	}
	if ranked[0].ProcessType != "b" {
		t.Errorf("第一名应为b (5音分), 实际=%s (%.1f)", ranked[0].ProcessType, ranked[0].DeviationCents)
	}
}

func TestCostBenefitRatio_Normal(t *testing.T) {
	result := models.ProcessComparisonResult{
		OverallScore: 0.8,
		CostScore:    0.5,
	}

	ratio := CostBenefitRatio(result)
	expected := 0.8 / 0.5
	if ratio != expected {
		t.Errorf("成本效益比应为%.2f, 实际=%.2f", expected, ratio)
	}
}

func TestCostBenefitRatio_ZeroCost_Boundary(t *testing.T) {
	result := models.ProcessComparisonResult{
		OverallScore: 0.8,
		CostScore:    0,
	}

	ratio := CostBenefitRatio(result)
	if ratio != 0 {
		t.Errorf("零成本时应返回0, 实际=%.2f", ratio)
	}
}

func TestTechniqueComparator_Reset_Normal(t *testing.T) {
	bell := &models.Bell{
		ID:              1,
		Name:            "test-bell",
		MassKg:          2.5,
		HeightCm:        30,
		DiameterCm:      20,
		ThicknessMm:     8.0,
		TargetFrequency: 440,
		ToleranceCents:  10,
	}

	tc := NewTechniqueComparator(bell)
	tc.Reset()

	results := tc.Compare(430, 440)
	if len(results) != 3 {
		t.Error("重置后应仍能正常对比")
	}
}

func TestTechniqueComparator_GetProcessSensitivity_Normal(t *testing.T) {
	bell := &models.Bell{
		ID:              1,
		Name:            "test-bell",
		MassKg:          2.5,
		HeightCm:        30,
		DiameterCm:      20,
		ThicknessMm:     8.0,
		TargetFrequency: 440,
		ToleranceCents:  10,
	}

	tc := NewTechniqueComparator(bell)
	pos := models.GrindingPosition{X: 0, Y: 0, Z: 0}
	sensitivity := tc.GetProcessSensitivity(ProcessGrinding, pos)

	if math.IsNaN(sensitivity) || math.IsInf(sensitivity, 0) {
		t.Errorf("灵敏度不应为NaN或Inf, 实际=%.6f", sensitivity)
	}
	t.Logf("磨锉灵敏度: %.6f Hz/mm", sensitivity)
}
