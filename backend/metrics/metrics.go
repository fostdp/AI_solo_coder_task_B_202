package metrics

import (
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bianzhong_http_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "route", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "bianzhong_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)

	MeasurementsReceived = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "bianzhong_measurements_received_total",
			Help: "Total acoustic measurements received from sensors",
		},
	)

	PitchDeviationAlerts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bianzhong_pitch_deviation_alerts_total",
			Help: "Total pitch deviation alerts triggered",
		},
		[]string{"severity"},
	)

	GrindingOperations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "bianzhong_grinding_operations_total",
			Help: "Total grinding operations recorded",
		},
	)

	SimulationRuns = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "bianzhong_simulation_duration_seconds",
			Help:    "FEM simulation computation duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
	)

	CorrectionOptimizations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "bianzhong_correction_iterations",
			Help:    "Pitch correction optimization iteration count and status",
			Buckets: []float64{1, 5, 10, 20, 50, 100, 200, 500},
		},
		[]string{"status"},
	)

	GridRebuilds = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bianzhong_grid_rebuilds_total",
			Help: "Total FEM grid rebuild operations with reason",
		},
		[]string{"reason", "success"},
	)

	ActiveWebSocketClients = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "bianzhong_websocket_clients_active",
			Help: "Number of active WebSocket client connections",
		},
	)

	MQTTDeliveries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bianzhong_mqtt_deliveries_total",
			Help: "MQTT alert delivery attempts with result",
		},
		[]string{"status"},
	)
)

type ResponseWriterWithStatus struct {
	http.ResponseWriter
	statusCode int
}

func (rws *ResponseWriterWithStatus) WriteHeader(code int) {
	rws.statusCode = code
	rws.ResponseWriter.WriteHeader(code)
}

func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		route := mux.CurrentRoute(r)
		var routeName string
		if route != nil {
			routeName, _ = route.GetPathTemplate()
		}
		if routeName == "" {
			routeName = r.URL.Path
		}

		rws := &ResponseWriterWithStatus{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rws, r)

		duration := time.Since(start).Seconds()
		HTTPRequestsTotal.WithLabelValues(
			r.Method,
			routeName,
			strconv.Itoa(rws.statusCode),
		).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, routeName).Observe(duration)
	})
}

func RegisterHandlers(r *mux.Router) {
	r.Handle("/metrics", promhttp.Handler()).Methods("GET")
	r.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
}
