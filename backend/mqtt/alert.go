package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	mqttclient "github.com/eclipse/paho.mqtt.golang"

	"bianzhong-acoustic-system/models"
)

type AlertManager struct {
	client   mqttclient.Client
	topicPrefix string
	enabled  bool
}

var GlobalAlertManager *AlertManager

func InitAlertManager() {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}

	clientID := os.Getenv("MQTT_CLIENT_ID")
	if clientID == "" {
		clientID = fmt.Sprintf("bianzhong-alert-%d", time.Now().Unix())
	}

	topicPrefix := os.Getenv("MQTT_TOPIC_PREFIX")
	if topicPrefix == "" {
		topicPrefix = "bianzhong/alerts"
	}

	enabledStr := os.Getenv("MQTT_ENABLED")
	enabled := enabledStr != "false"

	opts := mqttclient.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetKeepAlive(30 * time.Second)

	opts.OnConnect = func(c mqttclient.Client) {
		log.Println("MQTT connected successfully")
	}

	opts.OnConnectionLost = func(c mqttclient.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	}

	client := mqttclient.NewClient(opts)

	if enabled {
		token := client.Connect()
		token.WaitTimeout(5 * time.Second)
		if token.Error() != nil {
			log.Printf("MQTT connection failed (continuing without MQTT): %v", token.Error())
			enabled = false
		}
	}

	GlobalAlertManager = &AlertManager{
		client:      client,
		topicPrefix: topicPrefix,
		enabled:     enabled,
	}
}

func (am *AlertManager) SendAlert(alert *models.AlertEvent) (bool, error) {
	if !am.enabled {
		alert.MQTTTopic = ""
		alert.MQTTDelivered = false
		return false, nil
	}

	topic := fmt.Sprintf("%s/%s/%s", am.topicPrefix, alert.Severity, alert.AlertType)
	alert.MQTTTopic = topic

	payload, err := json.Marshal(alert)
	if err != nil {
		return false, fmt.Errorf("failed to marshal alert: %v", err)
	}

	token := am.client.Publish(topic, 1, false, payload)
	if token.WaitTimeout(5 * time.Second) {
		if token.Error() != nil {
			alert.MQTTDelivered = false
			return false, token.Error()
		}
		alert.MQTTDelivered = true
		return true, nil
	}

	alert.MQTTDelivered = false
	return false, fmt.Errorf("MQTT publish timeout")
}

func CreatePitchDeviationAlert(bell *models.Bell, measurement *models.AcousticMeasurement) *models.AlertEvent {
	return &models.AlertEvent{
		Time:      measurement.Time,
		BellID:    bell.ID,
		AlertType: "pitch_deviation",
		Severity:  "warning",
		Message: fmt.Sprintf("编钟 %s 音准偏差 %.2f 音分，超过容差 %.2f 音分",
			bell.Name, measurement.DeviationCents, bell.ToleranceCents),
		Details: map[string]interface{}{
			"current_frequency":  measurement.FundamentalFreq,
			"target_frequency":   bell.TargetFrequency,
			"deviation_cents":    measurement.DeviationCents,
			"tolerance_cents":    bell.ToleranceCents,
			"overtone_freqs":     measurement.OvertoneFreqs,
		},
	}
}

func CreateSeverePitchDeviationAlert(bell *models.Bell, measurement *models.AcousticMeasurement) *models.AlertEvent {
	return &models.AlertEvent{
		Time:      measurement.Time,
		BellID:    bell.ID,
		AlertType: "severe_pitch_deviation",
		Severity:  "critical",
		Message: fmt.Sprintf("编钟 %s 严重音准偏差 %.2f 音分（超过2倍容差）",
			bell.Name, measurement.DeviationCents),
		Details: map[string]interface{}{
			"current_frequency":  measurement.FundamentalFreq,
			"target_frequency":   bell.TargetFrequency,
			"deviation_cents":    measurement.DeviationCents,
			"tolerance_cents":    bell.ToleranceCents,
			"recommended_action": "immediate_correction",
		},
	}
}

func CreateGrindingExcessAlert(bell *models.Bell, operation *models.GrindingOperation, totalGrindedMm float64) *models.AlertEvent {
	return &models.AlertEvent{
		Time:      operation.Time,
		BellID:    bell.ID,
		AlertType: "grinding_excess",
		Severity:  "critical",
		Message: fmt.Sprintf("编钟 %s 磨削深度 %.2f mm 已超过最大允许值 %.2f mm",
			bell.Name, totalGrindedMm, bell.MaxGrindingDepthMm),
		Details: map[string]interface{}{
			"current_operation_depth": operation.GrindingDepthMm,
			"total_grinded_depth_mm":  totalGrindedMm,
			"max_allowed_depth_mm":    bell.MaxGrindingDepthMm,
			"position": map[string]float64{
				"x": operation.Position.X,
				"y": operation.Position.Y,
				"z": operation.Position.Z,
			},
		},
	}
}

func Close() {
	if GlobalAlertManager != nil && GlobalAlertManager.client != nil && GlobalAlertManager.enabled {
		GlobalAlertManager.client.Disconnect(250)
	}
}
