package metrics

type ReconcilerMetricsSet struct {
	ComponentProcessingDurationCollector *ComponentProcessingDurationMetric
}

func NewReconcilerMetricsSet(componentProcessingDurationCollector *ComponentProcessingDurationMetric) *ReconcilerMetricsSet {
	return &ReconcilerMetricsSet{ComponentProcessingDurationCollector: componentProcessingDurationCollector}
}
