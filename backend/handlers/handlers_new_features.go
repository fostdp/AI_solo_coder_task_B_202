package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"

	"bianzhong-acoustic-system/database"
	"bianzhong-acoustic-system/models"
	"bianzhong-acoustic-system/simulation"
)

var virtualSessions = make(map[string]*models.VirtualTuningSession)

func GetTuningProcesses(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(context.Background(), `
		SELECT id, name, process_type, description, harmonicity_factor,
		       frequency_shift_factor, reversibility, complexity, historical_era,
		       advantages, disadvantages
		FROM tuning_processes
		ORDER BY id
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	processes := make([]models.TuningProcess, 0)
	for rows.Next() {
		var p models.TuningProcess
		var advJSON, disadvJSON []byte
		err := rows.Scan(&p.ID, &p.Name, &p.ProcessType, &p.Description,
			&p.HarmonicityFactor, &p.FrequencyShiftFactor, &p.Reversibility,
			&p.Complexity, &p.HistoricalEra, &advJSON, &disadvJSON)
		if err != nil {
			continue
		}
		json.Unmarshal(advJSON, &p.Advantages)
		json.Unmarshal(disadvJSON, &p.Disadvantages)
		processes = append(processes, p)
	}

	respondJSON(w, http.StatusOK, processes)
}

func CompareTuningProcesses(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BellID       int      `json:"bell_id"`
		CurrentFreq  float64  `json:"current_freq"`
		TargetFreq   float64  `json:"target_freq"`
		ProcessTypes []string `json:"process_types,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.BellID == 0 {
		respondError(w, http.StatusBadRequest, "bell_id is required")
		return
	}

	var bell models.Bell
	err := database.DB.QueryRow(context.Background(), `
		SELECT id, name, serial_number, material, mass_kg, height_cm,
		       diameter_cm, thickness_mm, target_frequency, tolerance_cents,
		       max_grinding_depth_mm, created_at, description
		FROM bells WHERE id = $1
	`, req.BellID).Scan(&bell.ID, &bell.Name, &bell.SerialNumber, &bell.Material,
		&bell.MassKg, &bell.HeightCm, &bell.DiameterCm, &bell.ThicknessMm,
		&bell.TargetFrequency, &bell.ToleranceCents, &bell.MaxGrindingDepthMm,
		&bell.CreatedAt, &bell.Description)

	if err != nil {
		respondError(w, http.StatusNotFound, "Bell not found")
		return
	}

	if req.CurrentFreq <= 0 {
		req.CurrentFreq = bell.TargetFrequency * 0.98
	}
	if req.TargetFreq <= 0 {
		req.TargetFreq = bell.TargetFrequency
	}

	sim := simulation.NewFEMSimulator(&bell)
	results := sim.CompareTuningProcesses(req.CurrentFreq, req.TargetFreq)

	bestProcess := ""
	bestScore := -1.0
	for _, r := range results {
		if r.OverallScore > bestScore {
			bestScore = r.OverallScore
			bestProcess = r.ProcessType
		}
	}

	confidence := 0.0
	if len(results) > 0 {
		confidence = bestScore
	}

	resultsJSON, _ := json.Marshal(results)

	_, err = database.DB.Exec(context.Background(), `
		INSERT INTO process_comparisons (bell_id, process_types, results,
			best_process, confidence_score)
		VALUES ($1, $2, $3, $4, $5)
	`, req.BellID, req.ProcessTypes, resultsJSON, bestProcess, confidence)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"bell_id":      req.BellID,
		"current_freq": req.CurrentFreq,
		"target_freq":  req.TargetFreq,
		"best_process": bestProcess,
		"confidence":   confidence,
		"results":      results,
	})
}

func GetEmpiricalRules(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(context.Background(), `
		SELECT id, name, rule_text, source, historical_era,
		       formula, variables, description
		FROM empirical_rules
		ORDER BY id
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	rules := make([]models.EmpiricalRule, 0)
	for rows.Next() {
		var rule models.EmpiricalRule
		var varsJSON []byte
		err := rows.Scan(&rule.ID, &rule.Name, &rule.RuleText, &rule.Source,
			&rule.HistoricalEra, &rule.Formula, &varsJSON, &rule.Description)
		if err != nil {
			continue
		}
		json.Unmarshal(varsJSON, &rule.Variables)
		rules = append(rules, rule)
	}

	respondJSON(w, http.StatusOK, rules)
}

func ValidateEmpiricalRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RuleID int                    `json:"rule_id"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var rule models.EmpiricalRule
	var varsJSON []byte
	err := database.DB.QueryRow(context.Background(), `
		SELECT id, name, rule_text, source, historical_era,
		       formula, variables, description
		FROM empirical_rules WHERE id = $1
	`, req.RuleID).Scan(&rule.ID, &rule.Name, &rule.RuleText, &rule.Source,
		&rule.HistoricalEra, &rule.Formula, &varsJSON, &rule.Description)

	if err != nil {
		respondError(w, http.StatusNotFound, "Rule not found")
		return
	}
	json.Unmarshal(varsJSON, &rule.Variables)

	computed, expected, deviation, valid, pValue, ciLow, ciHigh, sig, effectSize, stdErr, confidence :=
		computeRuleValidation(rule, req.Params)

	sampleSize := 100
	if val, ok := req.Params["sample_size"]; ok {
		if f, ok := val.(float64); ok {
			sampleSize = int(f)
		}
	}

	result := models.RuleValidation{
		RuleID:                  rule.ID,
		ValidationResult:        valid,
		DeviationPercent:        deviation,
		ComputedValue:           computed,
		ExpectedValue:           expected,
		SampleSize:              sampleSize,
		Confidence:              confidence,
		PValue:                  pValue,
		ConfidenceIntervalLow:   ciLow,
		ConfidenceIntervalHigh:  ciHigh,
		StatisticalSignificance: sig,
		EffectSize:              effectSize,
		StandardError:           stdErr,
	}

	respondJSON(w, http.StatusOK, result)
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

func GetComparisonArticles(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	var rows pgx.Rows
	var err error
	args := []interface{}{}
	query := `
		SELECT id, title, category, bianzhong, piano, conclusion, references
		FROM comparison_articles
	`

	if category != "" {
		query += " WHERE category = $1"
		args = append(args, category)
	}

	query += " ORDER BY id"

	rows, err = database.DB.Query(context.Background(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	articles := make([]models.ComparisonArticle, 0)
	for rows.Next() {
		var a models.ComparisonArticle
		var bianzhongJSON, pianoJSON, refsJSON []byte
		err := rows.Scan(&a.ID, &a.Title, &a.Category, &bianzhongJSON, &pianoJSON, &a.Conclusion, &refsJSON)
		if err != nil {
			continue
		}
		json.Unmarshal(bianzhongJSON, &a.Bianzhong)
		json.Unmarshal(pianoJSON, &a.Piano)
		json.Unmarshal(refsJSON, &a.References)
		articles = append(articles, a)
	}

	respondJSON(w, http.StatusOK, articles)
}

func StartVirtualTuning(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BellID int `json:"bell_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var bell models.Bell
	err := database.DB.QueryRow(context.Background(), `
		SELECT id, name, target_frequency, tolerance_cents,
		       thickness_mm, max_grinding_depth_mm
		FROM bells WHERE id = $1
	`, req.BellID).Scan(&bell.ID, &bell.Name, &bell.TargetFrequency,
		&bell.ToleranceCents, &bell.ThicknessMm, &bell.MaxGrindingDepthMm)

	if err != nil {
		respondError(w, http.StatusNotFound, "Bell not found")
		return
	}

	sessionID := uuid.New().String()
	initialFreq := bell.TargetFrequency * (1 + (randFloat64()*0.04 - 0.02))

	session := &models.VirtualTuningSession{
		SessionID:    sessionID,
		BellID:       bell.ID,
		CurrentFreq:  initialFreq,
		OriginalFreq: initialFreq,
		TargetFreq:   bell.TargetFrequency,
		History:      make([]models.VirtualGrind, 0),
		TotalDepthMm: 0,
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
	}

	virtualSessions[sessionID] = session

	respondJSON(w, http.StatusCreated, session)
}

func VirtualTuningGrind(w http.ResponseWriter, r *http.Request) {
	sessionID := mux.Vars(r)["session_id"]
	session, ok := virtualSessions[sessionID]
	if !ok {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	var req struct {
		Position    models.GrindingPosition `json:"position"`
		DepthMm     float64                 `json:"depth_mm"`
		ProcessType string                  `json:"process_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.ProcessType == "" {
		req.ProcessType = "grinding"
	}

	var bell models.Bell
	database.DB.QueryRow(context.Background(), `
		SELECT id, name, target_frequency, tolerance_cents,
		       thickness_mm, max_grinding_depth_mm
		FROM bells WHERE id = $1
	`, session.BellID).Scan(&bell.ID, &bell.Name, &bell.TargetFrequency,
		&bell.ToleranceCents, &bell.ThicknessMm, &bell.MaxGrindingDepthMm)

	sim := simulation.NewFEMSimulator(&bell)

	sim.ApplyTuningProcess(req.ProcessType, req.Position, req.DepthMm)

	eigenfreqs := sim.CalculateEigenfrequencies()

	beforeFreq := session.CurrentFreq
	afterFreq := eigenfreqs[0]

	if req.ProcessType == "casting_inlay" || req.ProcessType == "welding_repair" {
		afterFreq = beforeFreq * (1 - req.DepthMm/bell.ThicknessMm*0.08)
	} else {
		afterFreq = beforeFreq * (1 + req.DepthMm/bell.ThicknessMm*0.12)
	}

	dev := 1200.0 * math.Log2(afterFreq/session.TargetFreq)

	grind := models.VirtualGrind{
		Time:        time.Now(),
		Position:    req.Position,
		DepthMm:     req.DepthMm,
		ProcessType: req.ProcessType,
		BeforeFreq:  beforeFreq,
		AfterFreq:   afterFreq,
		Deviation:   dev,
	}

	session.History = append(session.History, grind)
	session.CurrentFreq = afterFreq
	session.TotalDepthMm += math.Abs(req.DepthMm)
	session.LastModified = time.Now()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"session":          session,
		"grind_result":     grind,
		"eigenfreqs":       eigenfreqs,
		"harmonicity":      sim.CalculateHarmonicity(),
		"within_tolerance": math.Abs(dev) <= bell.ToleranceCents,
	})
}

func VirtualTuningPlay(w http.ResponseWriter, r *http.Request) {
	sessionID := mux.Vars(r)["session_id"]
	session, ok := virtualSessions[sessionID]
	if !ok {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	var bell models.Bell
	database.DB.QueryRow(context.Background(), `
		SELECT id, name, target_frequency, thickness_mm
		FROM bells WHERE id = $1
	`, session.BellID).Scan(&bell.ID, &bell.Name, &bell.TargetFrequency, &bell.ThicknessMm)

	sim := simulation.NewFEMSimulator(&bell)
	eigenfreqs := sim.CalculateEigenfrequencies()
	harmonicity := sim.CalculateHarmonicity()

	amplitudes := make([]float64, len(eigenfreqs))
	for i := range eigenfreqs {
		amplitudes[i] = math.Exp(-float64(i) * 0.4)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"session_id":   sessionID,
		"freqs":        eigenfreqs,
		"amplitudes":   amplitudes,
		"current_freq": session.CurrentFreq,
		"target_freq":  session.TargetFreq,
		"harmonicity":  harmonicity,
		"decay_rates":  []float64{1.5, 2.0, 2.8, 3.5, 4.2, 5.0, 5.8, 6.5},
	})
}

func VirtualTuningReset(w http.ResponseWriter, r *http.Request) {
	sessionID := mux.Vars(r)["session_id"]
	session, ok := virtualSessions[sessionID]
	if !ok {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	session.CurrentFreq = session.OriginalFreq
	session.History = make([]models.VirtualGrind, 0)
	session.TotalDepthMm = 0
	session.LastModified = time.Now()

	respondJSON(w, http.StatusOK, session)
}

func GetVirtualTuningSession(w http.ResponseWriter, r *http.Request) {
	sessionID := mux.Vars(r)["session_id"]
	session, ok := virtualSessions[sessionID]
	if !ok {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	respondJSON(w, http.StatusOK, session)
}

func randFloat64() float64 {
	return float64(time.Now().UnixNano()%10000) / 10000.0
}

func GetComparisonStats(w http.ResponseWriter, r *http.Request) {
	var totalComparisons int
	database.DB.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM process_comparisons").Scan(&totalComparisons)

	var grindingWins, castingWins, weldingWins int
	database.DB.QueryRow(context.Background(), `
		SELECT
			COUNT(*) FILTER (WHERE best_process = 'grinding'),
			COUNT(*) FILTER (WHERE best_process = 'casting_inlay'),
			COUNT(*) FILTER (WHERE best_process = 'welding_repair')
		FROM process_comparisons
	`).Scan(&grindingWins, &castingWins, &weldingWins)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_comparisons": totalComparisons,
		"best_process_counts": map[string]int{
			"grinding":       grindingWins,
			"casting_inlay":  castingWins,
			"welding_repair": weldingWins,
		},
	})
}
