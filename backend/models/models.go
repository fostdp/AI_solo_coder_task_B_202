package models

import (
	"time"
)

type Bell struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	SerialNumber        string    `json:"serial_number"`
	Material            string    `json:"material"`
	MassKg              float64   `json:"mass_kg"`
	HeightCm            float64   `json:"height_cm"`
	DiameterCm          float64   `json:"diameter_cm"`
	ThicknessMm         float64   `json:"thickness_mm"`
	TargetFrequency     float64   `json:"target_frequency"`
	ToleranceCents      float64   `json:"tolerance_cents"`
	MaxGrindingDepthMm  float64   `json:"max_grinding_depth_mm"`
	CreatedAt           time.Time `json:"created_at"`
	Description         string    `json:"description"`
}

type AcousticMeasurement struct {
	Time              time.Time `json:"time"`
	BellID            int       `json:"bell_id"`
	FundamentalFreq   float64   `json:"fundamental_freq"`
	OvertoneFreqs     []float64 `json:"overtone_freqs"`
	OvertoneAmplitudes []float64 `json:"overtone_amplitudes"`
	Temperature       float64   `json:"temperature"`
	Humidity          float64   `json:"humidity"`
	SensorID          string    `json:"sensor_id"`
	DeviationCents    float64   `json:"deviation_cents"`
}

type GrindingPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type GrindingOperation struct {
	ID                int              `json:"id"`
	Time              time.Time        `json:"time"`
	BellID            int              `json:"bell_id"`
	Position          GrindingPosition `json:"position"`
	GrindingDepthMm   float64          `json:"grinding_depth_mm"`
	GrindingArea      float64          `json:"grinding_area"`
	OperatorID        string           `json:"operator_id"`
	BeforeFrequency   float64          `json:"before_frequency"`
	AfterFrequency    float64          `json:"after_frequency"`
	PredictedFrequency float64         `json:"predicted_frequency"`
	Notes             string           `json:"notes"`
}

type CorrectionRecommendation struct {
	Position          GrindingPosition `json:"position"`
	DepthMm           float64          `json:"depth_mm"`
	Sensitivity       float64          `json:"sensitivity"`
	FrequencyChangeHz float64          `json:"frequency_change_hz"`
}

type PitchCorrection struct {
	ID                    int                        `json:"id"`
	CreatedAt             time.Time                  `json:"created_at"`
	BellID                int                        `json:"bell_id"`
	CurrentFrequency      float64                    `json:"current_frequency"`
	TargetFrequency       float64                    `json:"target_frequency"`
	DeviationCents        float64                    `json:"deviation_cents"`
	RecommendedPositions  []CorrectionRecommendation `json:"recommended_positions"`
	EstimatedResultFreq   float64                    `json:"estimated_result_freq"`
	Iterations            int                        `json:"iterations"`
	Algorithm             string                     `json:"algorithm"`
	Status                string                     `json:"status"`
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
	ID                  int                    `json:"id"`
	CreatedAt           time.Time              `json:"created_at"`
	BellID              int                    `json:"bell_id"`
	SimulationType      string                 `json:"simulation_type"`
	Parameters          map[string]interface{} `json:"parameters"`
	Eigenfrequencies    []float64              `json:"eigenfrequencies"`
	ModeShapes          map[string]interface{} `json:"mode_shapes"`
	StressDistribution  map[string]interface{} `json:"stress_distribution"`
	ComputationTimeMs   int64                  `json:"computation_time_ms"`
}

type ModeShapePoint struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Z          float64 `json:"z"`
	Displacement float64 `json:"displacement"`
	Stress     float64 `json:"stress"`
}
