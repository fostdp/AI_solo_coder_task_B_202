package empirical_rule_validator

import (
	"math"
	"testing"

	"bianzhong-acoustic-system/models"
)

func TestNewRuleValidator_Normal(t *testing.T) {
	rv := NewRuleValidator()
	if rv == nil {
		t.Fatal("RuleValidator should not be nil")
	}
}

func TestRuleValidator_Validate_Rule1_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rule := models.EmpiricalRule{ID: 1}
	params := map[string]interface{}{
		"thickness_mm": 8.0,
		"diameter_cm":  20.0,
	}

	result, err := rv.Validate(rule, params)
	if err != nil {
		t.Fatalf("验证不应出错: %v", err)
	}

	if result.RuleID != 1 {
		t.Errorf("RuleID应为1, 实际=%d", result.RuleID)
	}
	if result.ComputedValue <= 0 {
		t.Errorf("计算值应为正, 实际=%.4f", result.ComputedValue)
	}
	if result.ExpectedValue != result.ComputedValue {
		t.Error("无expected时expected应等于computed")
	}
	if result.ValidationResult != true {
		t.Error("无expected时验证应通过")
	}
}

func TestRuleValidator_Validate_Rule2_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rule := models.EmpiricalRule{ID: 2}
	params := map[string]interface{}{
		"mass_kg":   2.5,
		"height_cm": 30.0,
	}

	result, err := rv.Validate(rule, params)
	if err != nil {
		t.Fatalf("验证不应出错: %v", err)
	}

	if result.ComputedValue <= 0 {
		t.Errorf("Rule2计算值应为正, 实际=%.4f", result.ComputedValue)
	}
}

func TestRuleValidator_Validate_Rule3_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rule := models.EmpiricalRule{ID: 3}
	params := map[string]interface{}{
		"current_freq":    400.0,
		"grind_depth_mm":  0.5,
		"thickness_mm":    8.0,
	}

	result, err := rv.Validate(rule, params)
	if err != nil {
		t.Fatalf("验证不应出错: %v", err)
	}

	if result.ComputedValue <= 400 {
		t.Errorf("Rule3磨锉后频率应升高, 实际=%.2f", result.ComputedValue)
	}
}

func TestRuleValidator_Validate_Rule4_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rule := models.EmpiricalRule{ID: 4}
	params := map[string]interface{}{
		"diameter_cm": 20.0,
	}

	result, err := rv.Validate(rule, params)
	if err != nil {
		t.Fatalf("验证不应出错: %v", err)
	}

	expected := 8000.0 / 20.0
	if math.Abs(result.ComputedValue-expected) > 0.001 {
		t.Errorf("Rule4公式错误: 预期=%.2f, 实际=%.2f", expected, result.ComputedValue)
	}
}

func TestRuleValidator_Validate_Rule5_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rule := models.EmpiricalRule{ID: 5}
	params := map[string]interface{}{
		"lower_freq": 440.0,
	}

	result, err := rv.Validate(rule, params)
	if err != nil {
		t.Fatalf("验证不应出错: %v", err)
	}

	expected := 440.0 * math.Pow(2, 1.0/12.0)
	if math.Abs(result.ComputedValue-expected) > 0.01 {
		t.Errorf("Rule5半音间隔错误: 预期=%.2f, 实际=%.2f", expected, result.ComputedValue)
	}
}

func TestRuleValidator_Validate_UnknownRule_Abnormal(t *testing.T) {
	rv := NewRuleValidator()
	rule := models.EmpiricalRule{ID: 999}
	params := map[string]interface{}{}

	result, err := rv.Validate(rule, params)
	if err != nil {
		t.Fatalf("未知规则不应报错: %v", err)
	}

	if result.ComputedValue != 0 {
		t.Errorf("未知规则计算值应为0, 实际=%.4f", result.ComputedValue)
	}
	if result.ExpectedValue != 0 {
		t.Errorf("未知规则预期值应为0, 实际=%.4f", result.ExpectedValue)
	}
	if result.Confidence >= 0.8 {
		t.Errorf("未知规则置信度应较低, 实际=%.2f", result.Confidence)
	}
}

func TestRuleValidator_Validate_WithExpected_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rule := models.EmpiricalRule{ID: 1}
	params := map[string]interface{}{
		"thickness_mm": 8.0,
		"diameter_cm":  20.0,
		"expected":     10.0,
	}

	result, err := rv.Validate(rule, params)
	if err != nil {
		t.Fatalf("验证不应出错: %v", err)
	}

	if result.ExpectedValue != 10.0 {
		t.Errorf("预期值应为10.0, 实际=%.2f", result.ExpectedValue)
	}
	if result.DeviationPercent <= 0 {
		t.Errorf("偏差百分比应为正, 实际=%.2f", result.DeviationPercent)
	}
}

func TestRuleValidator_Validate_StatisticalFields_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rule := models.EmpiricalRule{ID: 1}
	params := map[string]interface{}{
		"thickness_mm": 8.0,
		"diameter_cm":  20.0,
		"sample_size":  float64(100),
	}

	result, err := rv.Validate(rule, params)
	if err != nil {
		t.Fatalf("验证不应出错: %v", err)
	}

	if result.PValue <= 0 || result.PValue > 1 {
		t.Errorf("p值应在0-1之间, 实际=%.6f", result.PValue)
	}
	if result.StandardError <= 0 {
		t.Errorf("标准误应为正, 实际=%.6f", result.StandardError)
	}
	if result.ConfidenceIntervalLow >= result.ConfidenceIntervalHigh {
		t.Error("置信区间下限应小于上限")
	}
}

func TestRuleValidator_ValidateAsync_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rule := models.EmpiricalRule{ID: 1}
	params := map[string]interface{}{
		"thickness_mm": 8.0,
		"diameter_cm":  20.0,
	}

	resultCh := rv.ValidateAsync(rule, params)
	result := <-resultCh

	if result.RuleID != 1 {
		t.Errorf("异步验证RuleID应为1, 实际=%d", result.RuleID)
	}
	if result.ComputedValue <= 0 {
		t.Error("异步验证计算值应为正")
	}
}

func TestRuleValidator_BatchValidate_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rules := []models.EmpiricalRule{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}
	params := map[string]interface{}{
		"thickness_mm":    8.0,
		"diameter_cm":     20.0,
		"mass_kg":         2.5,
		"height_cm":       30.0,
		"current_freq":    400.0,
		"grind_depth_mm":  0.5,
	}

	results := rv.BatchValidate(rules, params)
	if len(results) != 3 {
		t.Errorf("批量验证应有3个结果, 实际=%d", len(results))
	}
}

func TestRuleValidator_AverageConfidence_Normal(t *testing.T) {
	rv := NewRuleValidator()
	results := []models.RuleValidation{
		{Confidence: 0.8},
		{Confidence: 0.6},
		{Confidence: 0.7},
	}

	avg := rv.AverageConfidence(results)
	expected := (0.8 + 0.6 + 0.7) / 3.0
	if math.Abs(avg-expected) > 0.001 {
		t.Errorf("平均置信度应为%.3f, 实际=%.3f", expected, avg)
	}
}

func TestRuleValidator_AverageConfidence_Empty_Boundary(t *testing.T) {
	rv := NewRuleValidator()
	avg := rv.AverageConfidence([]models.RuleValidation{})
	if avg != 0 {
		t.Errorf("空列表平均置信度应为0, 实际=%.2f", avg)
	}
}

func TestRuleValidator_GetRuleByID_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rules := []models.EmpiricalRule{
		{ID: 1, Name: "Rule1"},
		{ID: 2, Name: "Rule2"},
	}

	found := rv.GetRuleByID(1, rules)
	if found == nil {
		t.Fatal("应找到ID=1的规则")
	}
	if found.ID != 1 {
		t.Errorf("找到的规则ID应为1, 实际=%d", found.ID)
	}

	notFound := rv.GetRuleByID(99, rules)
	if notFound != nil {
		t.Error("不存在的ID应返回nil")
	}
}

func TestRuleValidator_Validate_SampleSizeEffect_Normal(t *testing.T) {
	rv := NewRuleValidator()
	rule := models.EmpiricalRule{ID: 1}
	paramsSmall := map[string]interface{}{
		"thickness_mm": 8.0,
		"diameter_cm":  20.0,
		"sample_size":  float64(10),
	}
	paramsLarge := map[string]interface{}{
		"thickness_mm": 8.0,
		"diameter_cm":  20.0,
		"sample_size":  float64(1000),
	}

	resultSmall, _ := rv.Validate(rule, paramsSmall)
	resultLarge, _ := rv.Validate(rule, paramsLarge)

	if resultLarge.Confidence <= resultSmall.Confidence {
		t.Errorf("大样本置信度应更高: 小样本=%.2f, 大样本=%.2f",
			resultSmall.Confidence, resultLarge.Confidence)
	}
}
