package monitoring

import (
	"context"
	"testing"
	"time"
)

func TestCollectorOutput(t *testing.T) {
	collector := NewCollector()
	snap, err := collector.Collect()
	if err != nil {
		t.Fatalf("falha ao coletar métricas: %v", err)
	}

	if snap == nil {
		t.Fatal("snapshot não deve ser nulo")
	}

	if snap.CPU.Cores <= 0 {
		t.Errorf("esperado contagem de cores > 0, obtido %d", snap.CPU.Cores)
	}

	if snap.Memory.TotalBytes <= 0 {
		t.Errorf("esperado TotalBytes de memória > 0, obtido %d", snap.Memory.TotalBytes)
	}

	if snap.System.Hostname == "" {
		t.Error("hostname não deve ser vazio")
	}
}

func TestHistoryBuffer(t *testing.T) {
	buf := NewHistoryBuffer(3)

	if buf.Latest() != nil {
		t.Fatal("Latest() deve ser nil em buffer vazio")
	}

	snap1 := &ServerMetricsSnapshot{Timestamp: time.Now().Add(-2 * time.Second)}
	snap2 := &ServerMetricsSnapshot{Timestamp: time.Now().Add(-1 * time.Second)}
	snap3 := &ServerMetricsSnapshot{Timestamp: time.Now()}
	snap4 := &ServerMetricsSnapshot{Timestamp: time.Now().Add(1 * time.Second)}

	buf.Push(snap1)
	buf.Push(snap2)
	buf.Push(snap3)

	if len(buf.All()) != 3 {
		t.Fatalf("esperado 3 itens no buffer, obtido %d", len(buf.All()))
	}

	if buf.Latest() != snap3 {
		t.Fatal("Latest() deve retornar o último item inserido (snap3)")
	}

	// Inserir quarto item deve descartar snap1 mantendo capacidade = 3
	buf.Push(snap4)
	items := buf.All()
	if len(items) != 3 {
		t.Fatalf("esperado capacidade máxima 3 mantida, obtido %d", len(items))
	}
	if items[0] != snap2 {
		t.Fatal("primeiro item do buffer deve ser agora snap2")
	}
	if items[2] != snap4 {
		t.Fatal("último item deve ser snap4")
	}
}

func TestSamplerLifecycle(t *testing.T) {
	collector := NewCollector()
	history := NewHistoryBuffer(10)
	sampler := NewSampler(collector, history, 50*time.Millisecond)

	ctx := context.Background()
	sampler.Start(ctx)

	// Espera algumas coletas do sampler
	time.Sleep(160 * time.Millisecond)
	sampler.Stop()

	items := history.All()
	if len(items) < 2 {
		t.Fatalf("esperado ao menos 2 coletas no histórico pelo sampler, obtido %d", len(items))
	}
}
