package payment

import "sync"

type WaitHub struct {
	mu      sync.Mutex
	waiters map[string]chan PaymentResult
}

var DefaultWaitHub = NewWaitHub()

func NewWaitHub() *WaitHub {
	return &WaitHub{waiters: map[string]chan PaymentResult{}}
}

func (h *WaitHub) Register(orderNo string) (<-chan PaymentResult, func()) {
	if h == nil {
		h = DefaultWaitHub
	}
	ch := make(chan PaymentResult, 1)
	h.mu.Lock()
	h.waiters[orderNo] = ch
	h.mu.Unlock()
	cancel := func() {
		h.mu.Lock()
		if current := h.waiters[orderNo]; current == ch {
			delete(h.waiters, orderNo)
			close(ch)
		}
		h.mu.Unlock()
	}
	return ch, cancel
}

func (h *WaitHub) Resolve(orderNo string, result PaymentResult) bool {
	if h == nil {
		h = DefaultWaitHub
	}
	h.mu.Lock()
	ch := h.waiters[orderNo]
	if ch != nil {
		delete(h.waiters, orderNo)
	}
	h.mu.Unlock()
	if ch == nil {
		return false
	}
	ch <- result
	close(ch)
	return true
}
