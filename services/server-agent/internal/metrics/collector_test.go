package metrics

import (
	"testing"
)

func TestCollector(t *testing.T) {
	c := NewCollector()
	m, err := c.Collect()
	if err != nil {
		t.Fatalf("falha ao coletar métricas: %v", err)
	}

	if m == nil {
		t.Fatal("métricas retornadas são nulas")
	}

	if m.Timestamp.IsZero() {
		t.Error("timestamp das métricas não deve ser zero")
	}
}
