package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Event struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	OrderID  string `json:"order_id"`
	SellerID string `json:"seller_id"`
	BuyerID  string `json:"buyer_id"`
}

type Decision struct {
	EventID string `json:"event_id"`
	Action  string `json:"action"`
	OrderID string `json:"order_id"`
}

type Ledger struct {
	mu     sync.Mutex
	seen   map[string]bool
	assets map[string]bool
	buyers map[string]bool
}

func newLedger() *Ledger {
	return &Ledger{seen: map[string]bool{}, assets: map[string]bool{}, buyers: map[string]bool{}}
}

// Apply makes the handoff conditional on both upstream marketplace streams.
func (l *Ledger) Apply(e Event) (Decision, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	d := Decision{EventID: e.ID, OrderID: e.OrderID}
	if e.ID == "" || e.OrderID == "" {
		return d, errors.New("id and order_id are required")
	}
	if l.seen[e.ID] {
		d.Action = "already_applied"
		return d, nil
	}
	switch e.Kind {
	case "seller_asset":
		if e.SellerID == "" {
			return d, errors.New("seller_id is required")
		}
		l.assets[e.OrderID] = true
		d.Action = "asset_recorded"
	case "buyer_update":
		if e.BuyerID == "" {
			return d, errors.New("buyer_id is required")
		}
		l.buyers[e.OrderID] = true
		d.Action = "buyer_recorded"
	case "order_handoff":
		if !l.assets[e.OrderID] || !l.buyers[e.OrderID] {
			d.Action = "awaiting_inputs"
			return d, nil
		}
		d.Action = "handed_off"
	default:
		return d, fmt.Errorf("unknown kind %q", e.Kind)
	}
	l.seen[e.ID] = true
	return d, nil
}

type Server struct {
	api    *Client
	ledger *Ledger
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func respondError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		status := http.StatusBadGateway
		if apiErr.Status >= 400 && apiErr.Status < 500 {
			status = apiErr.Status
		}
		respond(w, status, map[string]any{"error": apiErr})
		return
	}
	respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func (s *Server) replay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		WebhookID string  `json:"webhook_id"`
		Events    []Event `json:"events"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		respondError(w, err)
		return
	}
	if input.WebhookID == "" || len(input.Events) == 0 {
		respondError(w, errors.New("webhook_id and events are required"))
		return
	}
	history, err := s.api.Deliveries(r.Context(), input.WebhookID)
	if err != nil {
		respondError(w, err)
		return
	}
	decisions := make([]Decision, 0, len(input.Events))
	for _, event := range input.Events {
		d, err := s.ledger.Apply(event)
		if err != nil {
			respondError(w, err)
			return
		}
		decisions = append(decisions, d)
	}
	respond(w, http.StatusOK, map[string]any{"decisions": decisions, "delivery_history": history})
}

func (s *Server) redrive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		Queue      string `json:"queue"`
		Batches    int    `json:"batches"`
		IntervalMS int    `json:"interval_ms"`
		RequestID  string `json:"request_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
		respondError(w, err)
		return
	}
	if strings.TrimSpace(input.Queue) == "" || input.Batches < 1 || input.Batches > 20 || input.IntervalMS < 1 || input.RequestID == "" {
		respondError(w, errors.New("queue, request_id, batches (1-20), and interval_ms (>0) are required"))
		return
	}
	results := make([]json.RawMessage, 0, input.Batches)
	for i := 0; i < input.Batches; i++ {
		if i > 0 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(time.Duration(input.IntervalMS) * time.Millisecond):
			}
		}
		result, err := s.api.Redrive(r.Context(), input.Queue, fmt.Sprintf("%s-%d", input.RequestID, i))
		if err != nil {
			respondError(w, err)
			return
		}
		results = append(results, result)
	}
	respond(w, http.StatusOK, map[string]any{"redrives": results})
}
