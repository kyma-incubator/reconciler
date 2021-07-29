package error

type ContextClosedError struct {
	Message string
}

func (m *ContextClosedError) Error() string {
	return m.Message
}
