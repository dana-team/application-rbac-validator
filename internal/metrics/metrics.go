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
		[]string{"name", "application_namespace", "destination_namespace", "destination", "reason"},
	)
)

// ObserveApplicationOptimizationStatus sets the optimization status metric for a given application.
func ObserveApplicationOptimizationStatus(name, appNamespace, destinationNamespace, destination, reason string, optimized bool) {
	value := map[bool]float64{true: 1, false: 0}[optimized]
	applicationOptimizationStatus.WithLabelValues(name, appNamespace, destinationNamespace, destination, reason).Set(value)
}

// DeleteApplicationOptimizationStatus deletes the optimization metric for the given application.
func DeleteApplicationOptimizationStatus(name, appNamespace, destination string) {
	applicationOptimizationStatus.DeleteLabelValues(name, appNamespace, destination)
}
