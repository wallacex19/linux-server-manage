package api

import (
	"encoding/json"
	"net/http"

	"github.com/linux-server-manager/backend/internal/monitoring"
)

type MetricsHandler struct {
	history   *monitoring.HistoryBuffer
	collector monitoring.Collector
}

func NewMetricsHandler(history *monitoring.HistoryBuffer, collector monitoring.Collector) *MetricsHandler {
	return &MetricsHandler{
		history:   history,
		collector: collector,
	}
}

func (h *MetricsHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	snap := h.history.Latest()
	if snap == nil {
		var err error
		snap, err = h.collector.Collect()
		if err != nil {
			http.Error(w, `{"error":"falha ao coletar métricas do servidor"}`, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(snap)
}

func (h *MetricsHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	items := h.history.All()
	if len(items) == 0 {
		// Se ainda não houve coletas de background, coleta ao menos 1 snapshot
		if snap, err := h.collector.Collect(); err == nil {
			h.history.Push(snap)
			items = []*monitoring.ServerMetricsSnapshot{snap}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(items)
}
