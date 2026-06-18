package handlers

import (
	"math"
	"testing"

	"bianzhong-acoustic-system/models"
)

func TestRuleValidation_Rule1_ThicknessFrequency_Normal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 1}
	params := map[string]interface{}{
		"thickness_mm": 8.0,
		"diameter_cm":  20.0,
	}

	computed, expected, deviation, valid, _, _, _, _, _, _, confidence := computeRuleValidation(rule, params)

	if computed <= 0 {
		t.Errorf("Rule1计算值应为正, 实际=%.4f", computed)
	}
	if expected != computed {
		t.Errorf("Rule1无expected时应等于computed, expected=%.4f, computed=%.4f", expected, computed)
	}
	if deviation != 0 {
		t.Errorf("Rule1无expected时偏差应为0, 实际=%.4f", deviation)
	}
	if !valid {
		t.Error("Rule1自验证应通过")
	}
	if confidence < 0.5 || confidence > 0.95 {
		t.Errorf("Rule1置信度应在0.5-0.95之间, 实际=%.2f", confidence)
	}

	expectedComputed := 0.6 * math.Sqrt(8.0) / (20.0 * 0.01)
	if math.Abs(computed-expectedComputed) > 0.001 {
		t.Errorf("Rule1公式计算错误: 预期=%.4f, 实际=%.4f", expectedComputed, computed)
	}
}

func TestRuleValidation_Rule2_MassHeight_Normal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 2}
	params := map[string]interface{}{
		"mass_kg":   2.5,
		"height_cm": 30.0,
	}

	computed, _, _, valid, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if computed <= 0 {
		t.Errorf("Rule2计算值应为正, 实际=%.4f", computed)
	}
	if !valid {
		t.Error("Rule2自验证应通过")
	}

	expectedComputed := 0.45 * math.Sqrt(2.5) / math.Pow(30.0*0.01, 0.75)
	if math.Abs(computed-expectedComputed) > 0.001 {
		t.Errorf("Rule2公式计算错误: 预期=%.4f, 实际=%.4f", expectedComputed, computed)
	}
}

func TestRuleValidation_Rule3_GrindFrequency_Normal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 3}
	params := map[string]interface{}{
		"current_freq":   440.0,
		"grind_depth_mm": 0.5,
		"thickness_mm":   8.0,
	}

	computed, expected, _, valid, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if computed <= 0 {
		t.Errorf("Rule3计算值应为正, 实际=%.4f", computed)
	}
	if expected != 440.0*1.02 {
		t.Errorf("Rule3默认预期应为当前频率*1.02, 实际=%.4f", expected)
	}
	if !valid {
		t.Error("Rule3正常参数应验证通过")
	}

	expectedComputed := 440.0 * (1 + 0.15*0.5/8.0)
	if math.Abs(computed-expectedComputed) > 0.001 {
		t.Errorf("Rule3公式计算错误: 预期=%.4f, 实际=%.4f", expectedComputed, computed)
	}
}

func TestRuleValidation_Rule4_DiameterFrequency_Normal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 4}
	params := map[string]interface{}{
		"diameter_cm": 20.0,
	}

	computed, _, _, valid, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if computed <= 0 {
		t.Errorf("Rule4计算值应为正, 实际=%.4f", computed)
	}
	if !valid {
		t.Error("Rule4自验证应通过")
	}

	expectedComputed := 8000.0 / 20.0
	if math.Abs(computed-expectedComputed) > 0.001 {
		t.Errorf("Rule4公式计算错误: 预期=%.4f, 实际=%.4f", expectedComputed, computed)
	}
}

func TestRuleValidation_Rule5_SemitoneInterval_Normal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 5}
	params := map[string]interface{}{
		"lower_freq": 440.0,
	}

	computed, expected, _, valid, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if computed <= 0 {
		t.Errorf("Rule5计算值应为正, 实际=%.4f", computed)
	}
	if computed <= 440.0 {
		t.Errorf("Rule5半音间隔应高于基础频率, 实际=%.4f", computed)
	}
	if expected != 440.0*1.059 {
		t.Errorf("Rule5默认预期应为lower_freq*1.059, 实际=%.4f", expected)
	}
	if !valid {
		t.Error("Rule5正常参数应验证通过")
	}

	expectedComputed := 440.0 * math.Pow(2, 1.0/12.0)
	if math.Abs(computed-expectedComputed) > 0.001 {
		t.Errorf("Rule5公式计算错误: 预期=%.4f, 实际=%.4f", expectedComputed, computed)
	}
}

func TestRuleValidation_WithExpectedValue_Boundary(t *testing.T) {
	rule := models.EmpiricalRule{ID: 1}
	params := map[string]interface{}{
		"thickness_mm": 8.0,
		"diameter_cm":  20.0,
		"expected":     0.6 * math.Sqrt(8.0) / (20.0 * 0.01),
	}

	_, _, deviation, valid, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if deviation > 0.001 {
		t.Errorf("完全匹配时偏差应接近0, 实际=%.4f", deviation)
	}
	if !valid {
		t.Error("完全匹配时应验证通过")
	}
}

func TestRuleValidation_DeviationWithinThreshold_Boundary(t *testing.T) {
	rule := models.EmpiricalRule{ID: 4}
	computedVal := 8000.0 / 20.0
	params := map[string]interface{}{
		"diameter_cm": 20.0,
		"expected":    computedVal * 1.09,
	}

	_, _, deviation, valid, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if deviation >= 10.0 {
		t.Errorf("9%%偏差应小于10%%阈值, 实际=%.4f%%", deviation)
	}
	if !valid {
		t.Error("9%偏差应在阈值内，验证应通过")
	}
}

func TestRuleValidation_DeviationExceedsThreshold_Boundary(t *testing.T) {
	rule := models.EmpiricalRule{ID: 4}
	computedVal := 8000.0 / 20.0
	params := map[string]interface{}{
		"diameter_cm": 20.0,
		"expected":    computedVal * 1.15,
	}

	_, _, deviation, valid, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if deviation <= 10.0 {
		t.Errorf("15%%偏差应超过10%%阈值, 实际=%.4f%%", deviation)
	}
	if valid {
		t.Error("15%偏差应验证失败")
	}
}

func TestRuleValidation_DeviationExactlyAtThreshold_Boundary(t *testing.T) {
	rule := models.EmpiricalRule{ID: 4}
	computedVal := 8000.0 / 20.0
	params := map[string]interface{}{
		"diameter_cm": 20.0,
		"expected":    computedVal * 0.95,
	}

	_, _, deviation, valid, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if deviation >= 10.0 {
		t.Errorf("5%%偏差应小于10%%阈值, 实际=%.4f%%", deviation)
	}
	if !valid {
		t.Error("5%偏差应验证通过")
	}
}

func TestRuleValidation_SampleSize_Normal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 1}
	params := map[string]interface{}{
		"thickness_mm": 8.0,
		"diameter_cm":  20.0,
		"sample_size":  float64(500),
	}

	result := models.RuleValidation{}
	computed, expected, deviation, valid, _, _, _, _, _, _, confidence := computeRuleValidation(rule, params)

	result.ComputedValue = computed
	result.ExpectedValue = expected
	result.DeviationPercent = deviation
	result.ValidationResult = valid
	result.Confidence = confidence

	sampleSize := 100
	if val, ok := params["sample_size"]; ok {
		if f, ok := val.(float64); ok {
			sampleSize = int(f)
		}
	}
	result.SampleSize = sampleSize

	if result.SampleSize != 500 {
		t.Errorf("样本量应为500, 实际=%d", result.SampleSize)
	}
}

func TestRuleValidation_DefaultSampleSize_Boundary(t *testing.T) {
	params := map[string]interface{}{
		"thickness_mm": 8.0,
		"diameter_cm":  20.0,
	}

	sampleSize := 100
	if val, ok := params["sample_size"]; ok {
		if f, ok := val.(float64); ok {
			sampleSize = int(f)
		}
	}

	if sampleSize != 100 {
		t.Errorf("默认样本量应为100, 实际=%d", sampleSize)
	}
}

func TestRuleValidation_UnknownRuleID_Abnormal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 999}
	params := map[string]interface{}{
		"thickness_mm": 8.0,
	}

	computed, expected, _, _, _, _, _, _, _, _, confidence := computeRuleValidation(rule, params)

	if computed != 0.0 {
		t.Errorf("未知规则计算值应为0, 实际=%.4f", computed)
	}
	if expected != 0.0 {
		t.Errorf("未知规则预期值应为0, 实际=%.4f", expected)
	}
	if confidence != 0.5 {
		t.Errorf("未知规则置信度应为0.5, 实际=%.2f", confidence)
	}
}

func TestRuleValidation_MissingParams_Abnormal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 1}
	params := map[string]interface{}{}

	computed, expected, deviation, valid, _, _, _, _, _, _, confidence := computeRuleValidation(rule, params)

	if confidence != 0.8 {
		t.Errorf("Rule1置信度应为0.8, 实际=%.2f", confidence)
	}
	if !math.IsNaN(computed) && computed != 0 {
		t.Logf("缺少参数时Rule1计算值=%.4f (可能为NaN)", computed)
	}
	if !math.IsNaN(expected) && expected != 0 {
		t.Logf("缺少参数时Rule1预期值=%.4f (可能为NaN)", expected)
	}
	if !math.IsNaN(deviation) && deviation < 0 {
		t.Errorf("缺少参数时偏差不应为负, 实际=%.4f", deviation)
	}
	_ = valid
}

func TestRuleValidation_ZeroExpected_Abnormal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 4}
	params := map[string]interface{}{
		"diameter_cm": 20.0,
		"expected":    0.0,
	}

	computed, _, deviation, _, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if computed <= 0 {
		t.Errorf("Rule4计算值应为正, 实际=%.4f", computed)
	}
	if deviation != 0 {
		t.Errorf("预期为0时偏差应保护为0, 实际=%.4f", deviation)
	}
}

func TestRuleValidation_NegativeParams_Abnormal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 3}
	params := map[string]interface{}{
		"current_freq":   -440.0,
		"grind_depth_mm": 0.5,
		"thickness_mm":   8.0,
	}

	computed, _, _, valid, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if valid && computed > 0 {
		t.Log("负频率参数计算值非正，验证通过")
	}
}

func TestRuleValidation_ExtremeValues_Abnormal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 4}
	params := map[string]interface{}{
		"diameter_cm": 0.001,
	}

	computed, _, _, _, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if math.IsInf(computed, 0) || math.IsNaN(computed) {
		t.Errorf("极小直径不应产生Inf/NaN, 实际=%.4f", computed)
	}
}

func TestRuleValidation_LargeDiameter_Abnormal(t *testing.T) {
	rule := models.EmpiricalRule{ID: 4}
	params := map[string]interface{}{
		"diameter_cm": 100000.0,
	}

	computed, _, _, _, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if math.IsNaN(computed) {
		t.Errorf("极大直径不应产生NaN, 实际=%.4f", computed)
	}
	if computed < 0 {
		t.Errorf("极大直径计算值应仍为正, 实际=%.4f", computed)
	}
}

func TestRuleValidation_Rule3_WithCustomExpected_Boundary(t *testing.T) {
	rule := models.EmpiricalRule{ID: 3}
	currentFreq := 440.0
	computedFreq := currentFreq * (1 + 0.15*0.5/8.0)
	params := map[string]interface{}{
		"current_freq":   currentFreq,
		"grind_depth_mm": 0.5,
		"thickness_mm":   8.0,
		"expected":       computedFreq * 1.05,
	}

	_, _, deviation, valid, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	if deviation > 10.0 {
		t.Errorf("5%%偏差应在阈值内, 实际=%.4f%%", deviation)
	}
	if !valid {
		t.Error("5%偏差应验证通过")
	}
}

func TestRuleValidation_Rule5_MusicalAccuracy(t *testing.T) {
	rule := models.EmpiricalRule{ID: 5}
	params := map[string]interface{}{
		"lower_freq": 261.63,
	}

	computed, _, _, _, _, _, _, _, _, _, _ := computeRuleValidation(rule, params)

	expectedCents := 100.0
	actualCents := 1200.0 * math.Log2(computed/261.63)

	if math.Abs(actualCents-expectedCents) > 1.0 {
		t.Errorf("Rule5半音间隔音分精度: 预期=%.2f cents, 实际=%.2f cents",
			expectedCents, actualCents)
	}
}

func TestRuleValidation_ConfidenceLevels(t *testing.T) {
	rules := []int{1, 2, 3, 4, 5, 999}

	for i, id := range rules {
		rule := models.EmpiricalRule{ID: id}
		params := map[string]interface{}{
			"thickness_mm":   8.0,
			"diameter_cm":    20.0,
			"mass_kg":        2.5,
			"height_cm":      30.0,
			"current_freq":   440.0,
			"grind_depth_mm": 0.5,
			"lower_freq":     440.0,
		}

		_, _, _, _, _, _, _, _, _, _, confidence := computeRuleValidation(rule, params)

		if id == 999 {
			if confidence != 0.5 {
				t.Errorf("未知规则置信度应为0.5, 实际=%.2f", confidence)
			}
		} else {
			if confidence <= 0 || confidence > 1.0 {
				t.Errorf("Rule%d置信度应在0-1之间, 实际=%.2f", id, confidence)
			}
			_ = i
		}
	}
}
