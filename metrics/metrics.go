package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds the system usage statistics.
type Metrics struct {
	Mu             sync.Mutex
	messageCount   int64
	lastCount      int64
	MessagesPerSec int64
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncrementMessages increments the message counter safely.
func (m *Metrics) IncrementMessages() {
	atomic.AddInt64(&m.messageCount, 1)
}

// StartTicker calculates messages processed per second.
func (m *Metrics) StartTicker() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			current := atomic.LoadInt64(&m.messageCount)
			
			// how many messages did we process in the last second?
			m.Mu.Lock()
			m.MessagesPerSec = current - m.lastCount
			m.lastCount = current
			m.Mu.Unlock()
		}
	}()
}
