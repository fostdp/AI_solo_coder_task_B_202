package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type MaterialParams struct {
	YoungModulus   float64 `json:"young_modulus_pa"`
	Density         float64 `json:"density_kg_m3"`
	PoissonRatio    float64 `json:"poisson_ratio"`
	SpeedOfSound    float64 `json:"speed_of_sound_m_s"`
}

type GridParams struct {
	Resolution             int     `json:"resolution"`
	MinThicknessRatio      float64 `json:"min_thickness_ratio"`
	MaxThicknessGradient   float64 `json:"max_thickness_gradient"`
	MaxDistortedRatio      float64 `json:"max_distorted_element_ratio"`
	RebuildSmoothingKernel int     `json:"rebuild_smoothing_kernel"`
	MaxRebuildRetries      int     `json:"max_rebuild_retries"`
	MinConnectedNodeRatio   float64 `json:"min_connected_node_ratio"`
}

type GrindingParams struct {
	GrindRadiusCm           float64 `json:"grind_radius_cm"`
	SensitivityPerturbation float64 `json:"sensitivity_perturbation_mm"`
}

type AcousticConfig struct {
	Materials                 map[string]MaterialParams `json:"materials"`
	Grid                      GridParams                `json:"grid"`
	Grinding                  GrindingParams            `json:"grinding"`
	HarmonicRatios            []float64                 `json:"harmonic_ratios"`
	FrequencyCalibrationFactor float64                 `json:"frequency_calibration_factor"`
	ThicknessExponent          float64                 `json:"thickness_exponent"`
}

type OptimizationParams struct {
	Algorithm           string  `json:"algorithm"`
	DefaultRho           float64 `json:"default_rho"`
	RhoIncreaseFactor    float64 `json:"rho_increase_factor"`
	MaxRho              float64 `json:"max_rho"`
	LambdaTolerance      float64 `json:"lambda_tolerance"`
	ConstraintTolerance  float64 `json:"constraint_tolerance"`
	MaxOuterIterations    int     `json:"max_outer_iterations"`
	MaxInnerIterations    int     `json:"max_inner_iterations"`
	LearningRate          float64 `json:"learning_rate"`
	ConvergenceFactor     float64 `json:"convergence_factor"`
}

type LineSearchParams struct {
	Alpha            float64 `json:"alpha"`
	Beta             float64 `json:"beta"`
	MaxBacktrackSteps int    `json:"max_backtrack_steps"`
}

type BoundaryParams struct {
	MinDepthPerPosition   float64 `json:"min_depth_per_position_mm"`
	MaxDepthPerPosition   float64 `json:"max_depth_per_position_mm"`
	MaxVariables           int     `json:"max_variables"`
	MinSensitivityScore   float64 `json:"min_sensitivity_score"`
}

type OscillationControlParams struct {
	BoundaryDampingZoneRatio  float64 `json:"boundary_damping_zone_ratio"`
	BoundaryDampingFactor     float64 `json:"boundary_damping_factor"`
	OscillationLookback       int     `json:"oscillation_lookback"`
	OscillationThresholdRatio float64 `json:"oscillation_threshold_ratio"`
}

type CandidatePositionParams struct {
	AngularSegments     int     `json:"angular_segments"`
	HeightLevels        int     `json:"height_levels"`
	RadiusFactorInner   float64 `json:"radius_factor_inner"`
	RadiusFactorOuter   float64 `json:"radius_factor_outer"`
}

type ConstraintConfig struct {
	Optimization      OptimizationParams       `json:"optimization"`
	LineSearch        LineSearchParams         `json:"line_search"`
	Boundaries        BoundaryParams           `json:"boundaries"`
	OscillationControl OscillationControlParams `json:"oscillation_control"`
	CandidatePositions CandidatePositionParams `json:"candidate_positions"`
}

var (
	instance *AppConfig
	once     sync.Once
)

type AppConfig struct {
	Acoustic   *AcousticConfig
	Constraint *ConstraintConfig
}

func Load(configDir string) (*AppConfig, error) {
	var loadErr error
	once.Do(func() {
		acousticPath := filepath.Join(configDir, "acoustic_params.json")
		constraintPath := filepath.Join(configDir, "constraint_params.json")

		acoustic, err := loadAcousticConfig(acousticPath)
		if err != nil {
			loadErr = fmt.Errorf("load acoustic config: %w", err)
			return
		}

		constraint, err := loadConstraintConfig(constraintPath)
		if err != nil {
			loadErr = fmt.Errorf("load constraint config: %w", err)
			return
		}

		instance = &AppConfig{
			Acoustic:   acoustic,
			Constraint: constraint,
		}
	})
	return instance, loadErr
}

func Get() *AppConfig {
	if instance == nil {
		_, _ = Load("./config")
	}
	return instance
}

func loadAcousticConfig(path string) (*AcousticConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	var cfg AcousticConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse json %s: %w", path, err)
	}
	return &cfg, nil
}

func loadConstraintConfig(path string) (*ConstraintConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	var cfg ConstraintConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse json %s: %w", path, err)
	}
	return &cfg, nil
}
