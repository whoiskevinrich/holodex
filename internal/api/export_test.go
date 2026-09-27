package api

// SetBootDrain pins the boot-drain flag (ADR-112 D2) so a test can observe a
// request arriving mid-drain without racing a real background drain.
func (h *Handlers) SetBootDrain(running bool) { h.bootDrain.Store(running) }
