package cluster

type MetricsCollectorMock struct{}

func (collector MetricsCollectorMock) OnClusterStateUpdate(_ *State) error {
	return nil
}
