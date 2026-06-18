package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4"

	"bianzhong-acoustic-system/database"
	"bianzhong-acoustic-system/models"
	"bianzhong-acoustic-system/mqtt"
	"bianzhong-acoustic-system/simulation"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var wsClients = make(map[*websocket.Conn]bool)

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func parseBellID(r *http.Request) (int, error) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	return strconv.Atoi(idStr)
}

func GetBells(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(context.Background(), `
		SELECT id, name, serial_number, material, mass_kg, height_cm,
		       diameter_cm, thickness_mm, target_frequency, tolerance_cents,
		       max_grinding_depth_mm, created_at, description
		FROM bells ORDER BY id
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	bells := make([]models.Bell, 0)
	for rows.Next() {
		var b models.Bell
		err := rows.Scan(&b.ID, &b.Name, &b.SerialNumber, &b.Material,
			&b.MassKg, &b.HeightCm, &b.DiameterCm, &b.ThicknessMm,
			&b.TargetFrequency, &b.ToleranceCents, &b.MaxGrindingDepthMm,
			&b.CreatedAt, &b.Description)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		bells = append(bells, b)
	}

	respondJSON(w, http.StatusOK, bells)
}

func GetBell(w http.ResponseWriter, r *http.Request) {
	id, err := parseBellID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid bell ID")
		return
	}

	var b models.Bell
	err = database.DB.QueryRow(context.Background(), `
		SELECT id, name, serial_number, material, mass_kg, height_cm,
		       diameter_cm, thickness_mm, target_frequency, tolerance_cents,
		       max_grinding_depth_mm, created_at, description
		FROM bells WHERE id = $1
	`, id).Scan(&b.ID, &b.Name, &b.SerialNumber, &b.Material,
		&b.MassKg, &b.HeightCm, &b.DiameterCm, &b.ThicknessMm,
		&b.TargetFrequency, &b.ToleranceCents, &b.MaxGrindingDepthMm,
		&b.CreatedAt, &b.Description)

	if err != nil {
		respondError(w, http.StatusNotFound, "Bell not found")
		return
	}

	respondJSON(w, http.StatusOK, b)
}

func PostAcousticMeasurement(w http.ResponseWriter, r *http.Request) {
	var m models.AcousticMeasurement
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var bell models.Bell
	err := database.DB.QueryRow(context.Background(), `
		SELECT id, name, target_frequency, tolerance_cents, max_grinding_depth_mm
		FROM bells WHERE id = $1
	`, m.BellID).Scan(&bell.ID, &bell.Name, &bell.TargetFrequency,
		&bell.ToleranceCents, &bell.MaxGrindingDepthMm)

	if err != nil {
		respondError(w, http.StatusNotFound, "Bell not found")
		return
	}

	if m.DeviationCents == 0 {
		m.DeviationCents = 1200.0 * math.Log2(m.FundamentalFreq/bell.TargetFrequency)
	}

	if m.Time.IsZero() {
		m.Time = time.Now()
	}

	_, err = database.DB.Exec(context.Background(), `
		INSERT INTO acoustic_measurements (time, bell_id, fundamental_freq,
			overtone_freqs, overtone_amplitudes, temperature, humidity,
			sensor_id, deviation_cents)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, m.Time, m.BellID, m.FundamentalFreq, m.OvertoneFreqs,
		m.OvertoneAmplitudes, m.Temperature, m.Humidity,
		m.SensorID, m.DeviationCents)

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if math.Abs(m.DeviationCents) > bell.ToleranceCents {
		var alert *models.AlertEvent
		if math.Abs(m.DeviationCents) > 2*bell.ToleranceCents {
			alert = mqtt.CreateSeverePitchDeviationAlert(&bell, &m)
		} else {
			alert = mqtt.CreatePitchDeviationAlert(&bell, &m)
		}

		delivered, _ := mqtt.GlobalAlertManager.SendAlert(alert)

		var detailsJSON []byte
		detailsJSON, _ = json.Marshal(alert.Details)

		database.DB.Exec(context.Background(), `
			INSERT INTO alert_events (time, bell_id, alert_type, severity,
				message, details, mqtt_topic, mqtt_delivered)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, alert.Time, alert.BellID, alert.AlertType, alert.Severity,
			alert.Message, detailsJSON, alert.MQTTTopic, delivered)
	}

	broadcastToWS(map[string]interface{}{
		"type":        "measurement",
		"data":        m,
		"bell_name":   bell.Name,
		"deviation":   m.DeviationCents,
		"target_freq": bell.TargetFrequency,
	})

	respondJSON(w, http.StatusCreated, m)
}

func GetAcousticMeasurements(w http.ResponseWriter, r *http.Request) {
	bellID, err := parseBellID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid bell ID")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	rows, err := database.DB.Query(context.Background(), `
		SELECT time, bell_id, fundamental_freq, overtone_freqs,
		       overtone_amplitudes, temperature, humidity, sensor_id, deviation_cents
		FROM acoustic_measurements
		WHERE bell_id = $1
		ORDER BY time DESC
		LIMIT $2
	`, bellID, limit)

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	measurements := make([]models.AcousticMeasurement, 0)
	for rows.Next() {
		var m models.AcousticMeasurement
		err := rows.Scan(&m.Time, &m.BellID, &m.FundamentalFreq,
			&m.OvertoneFreqs, &m.OvertoneAmplitudes, &m.Temperature,
			&m.Humidity, &m.SensorID, &m.DeviationCents)
		if err != nil {
			continue
		}
		measurements = append(measurements, m)
	}

	respondJSON(w, http.StatusOK, measurements)
}

func PostGrindingOperation(w http.ResponseWriter, r *http.Request) {
	var op models.GrindingOperation
	if err := json.NewDecoder(r.Body).Decode(&op); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var bell models.Bell
	err := database.DB.QueryRow(context.Background(), `
		SELECT id, name, target_frequency, tolerance_cents, max_grinding_depth_mm
		FROM bells WHERE id = $1
	`, op.BellID).Scan(&bell.ID, &bell.Name, &bell.TargetFrequency,
		&bell.ToleranceCents, &bell.MaxGrindingDepthMm)

	if err != nil {
		respondError(w, http.StatusNotFound, "Bell not found")
		return
	}

	if op.Time.IsZero() {
		op.Time = time.Now()
	}

	sim := simulation.NewFEMSimulator(&bell)
	if op.BeforeFrequency > 0 {
		sim.ApplyGrinding(op.Position, op.GrindingDepthMm)
		freqs := sim.CalculateEigenfrequencies()
		op.PredictedFrequency = freqs[0]
	}

	_, err = database.DB.Exec(context.Background(), `
		INSERT INTO grinding_operations (time, bell_id, position_x, position_y,
			position_z, grinding_depth_mm, grinding_area, operator_id,
			before_frequency, after_frequency, predicted_frequency, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, op.Time, op.BellID, op.Position.X, op.Position.Y, op.Position.Z,
		op.GrindingDepthMm, op.GrindingArea, op.OperatorID,
		op.BeforeFrequency, op.AfterFrequency, op.PredictedFrequency, op.Notes)

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var totalGrinded float64
	database.DB.QueryRow(context.Background(), `
		SELECT COALESCE(SUM(grinding_depth_mm), 0)
		FROM grinding_operations WHERE bell_id = $1
	`, op.BellID).Scan(&totalGrinded)

	if totalGrinded > bell.MaxGrindingDepthMm {
		alert := mqtt.CreateGrindingExcessAlert(&bell, &op, totalGrinded)
		delivered, _ := mqtt.GlobalAlertManager.SendAlert(alert)

		var detailsJSON []byte
		detailsJSON, _ = json.Marshal(alert.Details)

		database.DB.Exec(context.Background(), `
			INSERT INTO alert_events (time, bell_id, alert_type, severity,
				message, details, mqtt_topic, mqtt_delivered)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, alert.Time, alert.BellID, alert.AlertType, alert.Severity,
			alert.Message, detailsJSON, alert.MQTTTopic, delivered)
	}

	broadcastToWS(map[string]interface{}{
		"type":        "grinding",
		"data":        op,
		"bell_name":   bell.Name,
		"total_grinded_mm": totalGrinded,
	})

	respondJSON(w, http.StatusCreated, op)
}

func GetGrindingOperations(w http.ResponseWriter, r *http.Request) {
	bellID, err := parseBellID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid bell ID")
		return
	}

	rows, err := database.DB.Query(context.Background(), `
		SELECT id, time, bell_id, position_x, position_y, position_z,
		       grinding_depth_mm, grinding_area, operator_id,
		       before_frequency, after_frequency, predicted_frequency, notes
		FROM grinding_operations
		WHERE bell_id = $1
		ORDER BY time DESC
		LIMIT 100
	`, bellID)

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	ops := make([]models.GrindingOperation, 0)
	for rows.Next() {
		var op models.GrindingOperation
		err := rows.Scan(&op.ID, &op.Time, &op.BellID, &op.Position.X,
			&op.Position.Y, &op.Position.Z, &op.GrindingDepthMm,
			&op.GrindingArea, &op.OperatorID, &op.BeforeFrequency,
			&op.AfterFrequency, &op.PredictedFrequency, &op.Notes)
		if err != nil {
			continue
		}
		ops = append(ops, op)
	}

	respondJSON(w, http.StatusOK, ops)
}

func RunSimulation(w http.ResponseWriter, r *http.Request) {
	bellID, err := parseBellID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid bell ID")
		return
	}

	var req struct {
		SimulationType string                   `json:"simulation_type"`
		GrindingOps    []models.GrindingPosition `json:"grinding_operations,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.SimulationType == "" {
		req.SimulationType = "modal_analysis"
	}

	var bell models.Bell
	err = database.DB.QueryRow(context.Background(), `
		SELECT id, name, serial_number, material, mass_kg, height_cm,
		       diameter_cm, thickness_mm, target_frequency, tolerance_cents,
		       max_grinding_depth_mm, created_at, description
		FROM bells WHERE id = $1
	`, bellID).Scan(&bell.ID, &bell.Name, &bell.SerialNumber, &bell.Material,
		&bell.MassKg, &bell.HeightCm, &bell.DiameterCm, &bell.ThicknessMm,
		&bell.TargetFrequency, &bell.ToleranceCents, &bell.MaxGrindingDepthMm,
		&bell.CreatedAt, &bell.Description)

	if err != nil {
		respondError(w, http.StatusNotFound, "Bell not found")
		return
	}

	sim := simulation.NewFEMSimulator(&bell)

	for _, pos := range req.GrindingOps {
		sim.ApplyGrinding(pos, 0.5)
	}

	result := sim.RunSimulation(req.SimulationType)

	paramsJSON, _ := json.Marshal(result.Parameters)
	modeShapesJSON, _ := json.Marshal(result.ModeShapes)
	stressJSON, _ := json.Marshal(result.StressDistribution)

	_, err = database.DB.Exec(context.Background(), `
		INSERT INTO simulation_results (bell_id, simulation_type, parameters,
			eigenfrequencies, mode_shapes, stress_distribution, computation_time_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, result.BellID, result.SimulationType, paramsJSON,
		result.Eigenfrequencies, modeShapesJSON, stressJSON, result.ComputationTimeMs)

	respondJSON(w, http.StatusOK, result)
}

func GetPitchCorrection(w http.ResponseWriter, r *http.Request) {
	bellID, err := parseBellID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid bell ID")
		return
	}

	currentFreqStr := r.URL.Query().Get("current_freq")
	currentFreq, err := strconv.ParseFloat(currentFreqStr, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid current_freq parameter")
		return
	}

	var bell models.Bell
	err = database.DB.QueryRow(context.Background(), `
		SELECT id, name, serial_number, material, mass_kg, height_cm,
		       diameter_cm, thickness_mm, target_frequency, tolerance_cents,
		       max_grinding_depth_mm, created_at, description
		FROM bells WHERE id = $1
	`, bellID).Scan(&bell.ID, &bell.Name, &bell.SerialNumber, &bell.Material,
		&bell.MassKg, &bell.HeightCm, &bell.DiameterCm, &bell.ThicknessMm,
		&bell.TargetFrequency, &bell.ToleranceCents, &bell.MaxGrindingDepthMm,
		&bell.CreatedAt, &bell.Description)

	if err != nil {
		respondError(w, http.StatusNotFound, "Bell not found")
		return
	}

	optimizer := simulation.NewGradientDescentOptimizer(&bell)
	correction := optimizer.OptimizePitch(currentFreq, bell.TargetFrequency)

	recsJSON, _ := json.Marshal(correction.RecommendedPositions)

	_, err = database.DB.Exec(context.Background(), `
		INSERT INTO pitch_corrections (bell_id, current_frequency, target_frequency,
			deviation_cents, recommended_positions, estimated_result_freq,
			iterations, algorithm, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, correction.BellID, correction.CurrentFrequency, correction.TargetFrequency,
		correction.DeviationCents, recsJSON, correction.EstimatedResultFreq,
		correction.Iterations, correction.Algorithm, correction.Status)

	respondJSON(w, http.StatusOK, correction)
}

func GetAlerts(w http.ResponseWriter, r *http.Request) {
	bellIDStr := r.URL.Query().Get("bell_id")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	var rows pgx.Rows
	var err error

	query := `
		SELECT id, time, bell_id, alert_type, severity, message,
		       details, acknowledged, mqtt_topic, mqtt_delivered
		FROM alert_events
	`
	args := []interface{}{}

	if bellIDStr != "" {
		bellID, _ := strconv.Atoi(bellIDStr)
		query += " WHERE bell_id = $1"
		args = append(args, bellID)
	}

	query += " ORDER BY time DESC LIMIT $" + fmt.Sprintf("%d", len(args)+1)
	args = append(args, limit)

	rows, err = database.DB.Query(context.Background(), query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	alerts := make([]models.AlertEvent, 0)
	for rows.Next() {
		var a models.AlertEvent
		var detailsJSON []byte
		err := rows.Scan(&a.ID, &a.Time, &a.BellID, &a.AlertType,
			&a.Severity, &a.Message, &detailsJSON, &a.Acknowledged,
			&a.MQTTTopic, &a.MQTTDelivered)
		if err != nil {
			continue
		}
		json.Unmarshal(detailsJSON, &a.Details)
		alerts = append(alerts, a)
	}

	respondJSON(w, http.StatusOK, alerts)
}

func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	wsClients[conn] = true
	log.Println("New WebSocket client connected")

	defer func() {
		delete(wsClients, conn)
		conn.Close()
		log.Println("WebSocket client disconnected")
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func broadcastToWS(data interface{}) {
	msg, _ := json.Marshal(data)
	for client := range wsClients {
		err := client.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			client.Close()
			delete(wsClients, client)
		}
	}
}

func GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	var totalBells int
	database.DB.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM bells").Scan(&totalBells)

	var totalMeasurements int
	database.DB.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM acoustic_measurements WHERE time > NOW() - INTERVAL '24 hours'").Scan(&totalMeasurements)

	var activeAlerts int
	database.DB.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM alert_events WHERE acknowledged = false AND time > NOW() - INTERVAL '24 hours'").Scan(&activeAlerts)

	var totalGrindingOps int
	database.DB.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM grinding_operations WHERE time > NOW() - INTERVAL '24 hours'").Scan(&totalGrindingOps)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_bells":          totalBells,
		"measurements_24h":     totalMeasurements,
		"active_alerts":        activeAlerts,
		"grinding_ops_24h":     totalGrindingOps,
	})
}
