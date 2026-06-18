package models

import (
	"time"
)

type Bell struct {
	ID                 int       `json:"id"`
	Name               string    `json:"name"`
	SerialNumber       string    `json:"serial_number"`
	Material           string    `json:"material"`
	MassKg             float64   `json:"mass_kg"`
	HeightCm           float64   `json:"height_cm"`
	DiameterCm         float64   `json:"diameter_cm"`
	ThicknessMm        float64   `json:"thickness_mm"`
	TargetFrequency    float64   `json:"target_frequency"`
	ToleranceCents     float64   `json:"tolerance_cents"`
	MaxGrindingDepthMm float64   `json:"max_grinding_depth_mm"`
	CreatedAt          time.Time `json:"created_at"`
	Description        string    `json:"description"`
}

type AcousticMeasurement struct {
	Time               time.Time `json:"time"`
	BellID             int       `json:"bell_id"`
	FundamentalFreq    float64   `json:"fundamental_freq"`
	OvertoneFreqs      []float64 `json:"overtone_freqs"`
	OvertoneAmplitudes []float64 `json:"overtone_amplitudes"`
	Temperature        float64   `json:"temperature"`
	Humidity           float64   `json:"humidity"`
	SensorID           string    `json:"sensor_id"`
	DeviationCents     float64   `json:"deviation_cents"`
}

type GrindingPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type GrindingOperation struct {
	ID                 int              `json:"id"`
	Time               time.Time        `json:"time"`
	BellID             int              `json:"bell_id"`
	Position           GrindingPosition `json:"position"`
	GrindingDepthMm    float64          `json:"grinding_depth_mm"`
	GrindingArea       float64          `json:"grinding_area"`
	OperatorID         string           `json:"operator_id"`
	BeforeFrequency    float64          `json:"before_frequency"`
	AfterFrequency     float64          `json:"after_frequency"`
	PredictedFrequency float64          `json:"predicted_frequency"`
	Notes              string           `json:"notes"`
}

type CorrectionRecommendation struct {
	Position          GrindingPosition `json:"position"`
	DepthMm           float64          `json:"depth_mm"`
	Sensitivity       float64          `json:"sensitivity"`
	FrequencyChangeHz float64          `json:"frequency_change_hz"`
}

type PitchCorrection struct {
	ID                   int                        `json:"id"`
	CreatedAt            time.Time                  `json:"created_at"`
	BellID               int                        `json:"bell_id"`
	CurrentFrequency     float64                    `json:"current_frequency"`
	TargetFrequency      float64                    `json:"target_frequency"`
	DeviationCents       float64                    `json:"deviation_cents"`
	RecommendedPositions []CorrectionRecommendation `json:"recommended_positions"`
	EstimatedResultFreq  float64                    `json:"estimated_result_freq"`
	Iterations           int                        `json:"iterations"`
	Algorithm            string                     `json:"algorithm"`
	Status               string                     `json:"status"`
}

type AlertEvent struct {
	ID            int                    `json:"id"`
	Time          time.Time              `json:"time"`
	BellID        int                    `json:"bell_id"`
	AlertType     string                 `json:"alert_type"`
	Severity      string                 `json:"severity"`
	Message       string                 `json:"message"`
	Details       map[string]interface{} `json:"details"`
	Acknowledged  bool                   `json:"acknowledged"`
	MQTTTopic     string                 `json:"mqtt_topic"`
	MQTTDelivered bool                   `json:"mqtt_delivered"`
}

type SimulationResult struct {
	ID                 int                    `json:"id"`
	CreatedAt          time.Time              `json:"created_at"`
	BellID             int                    `json:"bell_id"`
	SimulationType     string                 `json:"simulation_type"`
	Parameters         map[string]interface{} `json:"parameters"`
	Eigenfrequencies   []float64              `json:"eigenfrequencies"`
	ModeShapes         map[string]interface{} `json:"mode_shapes"`
	StressDistribution map[string]interface{} `json:"stress_distribution"`
	ComputationTimeMs  int64                  `json:"computation_time_ms"`
}

type ModeShapePoint struct {
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Z            float64 `json:"z"`
	Displacement float64 `json:"displacement"`
	Stress       float64 `json:"stress"`
}

type TuningProcess struct {
	ID                   int      `json:"id"`
	Name                 string   `json:"name"`
	ProcessType          string   `json:"process_type"`
	Description          string   `json:"description"`
	HarmonicityFactor    float64  `json:"harmonicity_factor"`
	FrequencyShiftFactor float64  `json:"frequency_shift_factor"`
	Reversibility        bool     `json:"reversibility"`
	Complexity           int      `json:"complexity"`
	HistoricalEra        string   `json:"historical_era"`
	Advantages           []string `json:"advantages"`
	Disadvantages        []string `json:"disadvantages"`
}

type ProcessComparison struct {
	ID              int                    `json:"id"`
	CreatedAt       time.Time              `json:"created_at"`
	BellID          int                    `json:"bell_id"`
	ProcessTypes    []string               `json:"process_types"`
	Results         map[string]interface{} `json:"results"`
	BestProcess     string                 `json:"best_process"`
	ConfidenceScore float64                `json:"confidence_score"`
}

type ProcessComparisonResult struct {
	ProcessType     string  `json:"process_type"`
	EstimatedFreq   float64 `json:"estimated_freq"`
	FreqDeltaHz     float64 `json:"freq_delta_hz"`
	DeviationCents  float64 `json:"deviation_cents"`
	Harmonicity     float64 `json:"harmonicity"`
	Complexity      int     `json:"complexity"`
	Reversibility   bool    `json:"reversibility"`
	DamageRisk      float64 `json:"damage_risk"`
	RequiredTimeMin int     `json:"required_time_min"`
	CostScore       float64 `json:"cost_score"`
	OverallScore    float64 `json:"overall_score"`
}

type EmpiricalRule struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	RuleText      string   `json:"rule_text"`
	Source        string   `json:"source"`
	HistoricalEra string   `json:"historical_era"`
	Formula       string   `json:"formula"`
	Variables     []string `json:"variables"`
	Description   string   `json:"description"`
}

type RuleValidation struct {
	RuleID           int     `json:"rule_id"`
	ValidationResult bool    `json:"validation_result"`
	DeviationPercent float64 `json:"deviation_percent"`
	ComputedValue    float64 `json:"computed_value"`
	ExpectedValue    float64 `json:"expected_value"`
	SampleSize       int     `json:"sample_size"`
	Confidence       float64 `json:"confidence"`
}

type ComparisonArticle struct {
	ID         int                    `json:"id"`
	Title      string                 `json:"title"`
	Category   string                 `json:"category"`
	Bianzhong  map[string]interface{} `json:"bianzhong"`
	Piano      map[string]interface{} `json:"piano"`
	Conclusion string                 `json:"conclusion"`
	References []string               `json:"references"`
}

type VirtualTuningSession struct {
	SessionID    string         `json:"session_id"`
	BellID       int            `json:"bell_id"`
	CurrentFreq  float64        `json:"current_freq"`
	OriginalFreq float64        `json:"original_freq"`
	TargetFreq   float64        `json:"target_freq"`
	History      []VirtualGrind `json:"history"`
	TotalDepthMm float64        `json:"total_depth_mm"`
	CreatedAt    time.Time      `json:"created_at"`
	LastModified time.Time      `json:"last_modified"`
}

type VirtualGrind struct {
	Time        time.Time        `json:"time"`
	Position    GrindingPosition `json:"position"`
	DepthMm     float64          `json:"depth_mm"`
	ProcessType string           `json:"process_type"`
	BeforeFreq  float64          `json:"before_freq"`
	AfterFreq   float64          `json:"after_freq"`
	Deviation   float64          `json:"deviation"`
}
