package monitoring

import (
	"context"
	"log"
	"time"
)

type Collector interface {
	Collect() (*ServerMetricsSnapshot, error)
}

type Sampler struct {
	collector Collector
	history   *HistoryBuffer
	interval  time.Duration
	cancel    context.CancelFunc
}

func NewSampler(collector Collector, history *HistoryBuffer, interval time.Duration) *Sampler {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return &Sampler{
		collector: collector,
		history:   history,
		interval:  interval,
	}
}

func (s *Sampler) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Coleta inicial síncrona para que a primeira requisição já tenha dados
	if snap, err := s.collector.Collect(); err == nil {
		s.history.Push(snap)
	}

	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snap, err := s.collector.Collect()
				if err != nil {
					log.Printf("[Monitoring Sampler] Erro na coleta: %v", err)
					continue
				}
				s.history.Push(snap)
			}
		}
	}()
}

func (s *Sampler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}
