package events

// SetFanOutHook sets a function that is called by fanOut before dispatching
// each event. Pass nil to disable. Used only in tests.
func SetFanOutHook(fn func()) {
	fanOutHookMu.Lock()
	fanOutHook = fn
	fanOutHookMu.Unlock()
}
