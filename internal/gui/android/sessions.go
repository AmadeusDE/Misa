package android

// SessionIDs returns a snapshot in creation order, without revealing any session.
func (m *Manager) SessionIDs() []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]int(nil), m.order...)
}
