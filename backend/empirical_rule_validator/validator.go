package empirical_rule_validator

import (
	"fmt"
	"math"

	"bianzhong-acoustic-system/models"
)

type RuleValidator struct{}

func NewRuleValidator() *RuleValidator {
	return &RuleValidator{}
}

func (rv *RuleValidator) Validate(rule models.EmpiricalRule, params map[string]interface{}) (models.RuleValidation, error) {
	computed, expected, deviation, valid,
		pValue, ciLow, ciHigh, statSig, effectSize, stdErr, confidence := computeRuleValidation(rule, params)

	if math.IsNaN(computed) || math.IsInf(computed, 0) {
		return models.RuleValidation{}, fmt.Errorf("invalid computed value")
	}

	sampleSize := 100
	if val, ok := params["sample_size"]; ok {
		if f, ok := val.(float64); ok {
			sampleSize = int(f)
		}
	}

	return models.RuleValidation{
		RuleID:                rule.ID,
		ComputedValue:         computed,
		ExpectedValue:         expected,
		DeviationPercent:      deviation,
		ValidationResult:      valid,
		Confidence:            confidence,
		SampleSize:            sampleSize,
		PValue:                pValue,
		ConfidenceIntervalLow: ciLow,
		ConfidenceIntervalHigh: ciHigh,
		StatisticalSignificance: statSig,
		EffectSize:            effectSize,
		StandardError:         stdErr,
	}, nil
}

func (rv *RuleValidator) ValidateAsync(rule models.EmpiricalRule, params map[string]interface{}) <-chan models.RuleValidation {
	resultCh := make(chan models.RuleValidation, 1)
	go func() {
		defer close(resultCh)
		result, _ := rv.Validate(rule, params)
		resultCh <- result
	}()
	return resultCh
}

func (rv *RuleValidator) BatchValidate(rules []models.EmpiricalRule, params map[string]interface{}) []models.RuleValidation {
	results := make([]models.RuleValidation, 0, len(rules))
	for _, rule := range rules {
		if result, err := rv.Validate(rule, params); err == nil {
			results = append(results, result)
		}
	}
	return results
}

func (rv *RuleValidator) GetRuleByID(id int, allRules []models.EmpiricalRule) *models.EmpiricalRule {
	for _, rule := range allRules {
		if rule.ID == id {
			return &rule
		}
	}
	return nil
}

func (rv *RuleValidator) AverageConfidence(results []models.RuleValidation) float64 {
	if len(results) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range results {
		sum += r.Confidence
	}
	return sum / float64(len(results))
}

func computeRuleValidation(rule models.EmpiricalRule, params map[string]interface{}) (
	float64, float64, float64, bool,
	float64, float64, float64, bool, float64, float64, float64) {

	expected := 0.0
	computed := 0.0
	baseConfidence := 0.8

	sampleSize := 100
	if val, ok := params["sample_size"]; ok {
		if f, ok := val.(float64); ok {
			sampleSize = int(f)
		}
	}

	if val, ok := params["expected"]; ok {
		expected, _ = val.(float64)
	}

	switch rule.ID {
	case 1:
		thickness, _ := params["thickness_mm"].(float64)
		diameter, _ := params["diameter_cm"].(float64)
		computed = 0.6 * math.Sqrt(thickness) / (diameter * 0.01)
		if expected <= 0 {
			expected = computed
		}
	case 2:
		mass, _ := params["mass_kg"].(float64)
		height, _ := params["height_cm"].(float64)
		computed = 0.45 * math.Sqrt(mass) / math.Pow(height*0.01, 0.75)
		if expected <= 0 {
			expected = computed
		}
	case 3:
		currentFreq, _ := params["current_freq"].(float64)
		grindDepth, _ := params["grind_depth_mm"].(float64)
		thickness, _ := params["thickness_mm"].(float64)
		computed = currentFreq * (1 + 0.15*grindDepth/thickness)
		if expected <= 0 {
			expected = currentFreq * 1.02
		}
	case 4:
		diameter, _ := params["diameter_cm"].(float64)
		computed = 8000.0 / diameter
		if expected <= 0 {
			expected = computed
		}
	case 5:
		lowerFreq, _ := params["lower_freq"].(float64)
		computed = lowerFreq * math.Pow(2, 1.0/12.0)
		if expected <= 0 {
			expected = lowerFreq * 1.059
		}
	default:
		computed = 0.0
		expected = 0.0
		baseConfidence = 0.5
	}

	deviation := 0.0
	if expected > 0 {
		deviation = math.Abs(computed-expected) / expected * 100
	}

	valid := deviation < 10.0

	effectSize := 0.0
	stdErr := 0.0
	pValue := 1.0
	ciLow := 0.0
	ciHigh := 0.0
	statSig := false
	confidence := baseConfidence

	if expected > 0 && sampleSize > 1 {
		effectSize = math.Abs(computed-expected) / expected

		stdErr = expected * 0.05 / math.Sqrt(float64(sampleSize))

		zScore := 0.0
		if stdErr > 0 {
			zScore = math.Abs(computed-expected) / stdErr
		}
		pValue = 2 * (1.0 - normalCDF(zScore))

		z95 := 1.96
		marginOfError := z95 * stdErr
		ciLow = computed - marginOfError
		ciHigh = computed + marginOfError

		statSig = pValue < 0.05

		sampleFactor := math.Min(1.0, math.Log(float64(sampleSize))/math.Log(1000))
		deviationPenalty := math.Min(1.0, deviation/20.0)
		confidence = baseConfidence*0.6 + sampleFactor*0.25 - deviationPenalty*0.2
		confidence = math.Max(0.1, math.Min(0.99, confidence))
	}

	return computed, expected, deviation, valid,
		pValue, ciLow, ciHigh, statSig, effectSize, stdErr, confidence
}

func normalCDF(x float64) float64 {
	return 0.5 * (1.0 + math.Erf(x/math.Sqrt2))
}
