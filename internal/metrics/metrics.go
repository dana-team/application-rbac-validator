package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func InitializeMetrics() {
	metrics.Registry.MustRegister(
		applicationOptimizationStatus,
	)
}

var (
	applicationOptimizationStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "application_optimization_status",
			Help: "Indicates whether the application is optimized (1) or not (0)",
		},
		[]string{"name", "destination_namespace", "destination", "reason"},
	)
)

// ObserveApplicationOptimizationStatus sets the optimization status metric for a given application.
func ObserveApplicationOptimizationStatus(name, namespace, destination, reason string, optimized bool) {
	value := map[bool]float64{true: 1, false: 0}[optimized]
	applicationOptimizationStatus.WithLabelValues(name, namespace, destination, reason).Set(value)
}

// DeleteApplicationOptimizationStatus deletes the optimization metric for the given application.
func DeleteApplicationOptimizationStatus(name, namespace, destination string) {
	applicationOptimizationStatus.DeleteLabelValues(name, namespace, destination)
}
