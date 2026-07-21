package web

const (
	maxPluginWebRequestBody = 4 << 20
	maxPluginExecutions     = 8
)

func (s *Server) acquirePluginExecution() func() {
	s.pluginExecutionMu.Lock()
	if s.pluginExecutionSlots == nil {
		s.pluginExecutionSlots = make(chan struct{}, maxPluginExecutions)
	}
	slots := s.pluginExecutionSlots
	s.pluginExecutionMu.Unlock()

	select {
	case slots <- struct{}{}:
		return func() { <-slots }
	default:
		return nil
	}
}
