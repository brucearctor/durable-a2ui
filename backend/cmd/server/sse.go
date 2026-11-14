package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/workflowstreams"
	"go.temporal.io/sdk/converter"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

// SubscriberTracker tracks active SSE browser/phone connections per workflow.
type SubscriberTracker struct {
	mu      sync.Mutex
	clients map[string]map[chan int]struct{}
}

func NewSubscriberTracker() *SubscriberTracker {
	return &SubscriberTracker{
		clients: make(map[string]map[chan int]struct{}),
	}
}

func (st *SubscriberTracker) Join(workflowID string) (chan int, int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	ch := make(chan int, 8)
	if st.clients[workflowID] == nil {
		st.clients[workflowID] = make(map[chan int]struct{})
	}
	st.clients[workflowID][ch] = struct{}{}
	count := len(st.clients[workflowID])
	for c := range st.clients[workflowID] {
		select {
		case c <- count:
		default:
		}
	}
	return ch, count
}

func (st *SubscriberTracker) Leave(workflowID string, ch chan int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if m, ok := st.clients[workflowID]; ok {
		delete(m, ch)
		count := len(m)
		if count == 0 {
			delete(st.clients, workflowID)
		} else {
			for c := range m {
				select {
				case c <- count:
				default:
				}
			}
		}
	}
}

// NewTopicSSEHandler creates an HTTP Server-Sent Events handler that streams
// Temporal workflowstreams topics and falls back to the a2ui_stream_state
// Temporal Query Handler when a workflow has already reached COMPLETED state.
func NewTopicSSEHandler(tc client.Client, subTracker *SubscriberTracker) a2ui.TopicSSEHandler {
	return func(w http.ResponseWriter, r *http.Request, workflowID string, topic string) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		lastEventID := -1
		if q := r.URL.Query().Get("last_event_id"); q != "" {
			if v, err := strconv.Atoi(q); err == nil {
				lastEventID = v
			}
		} else if h := r.Header.Get("Last-Event-ID"); h != "" {
			if v, err := strconv.Atoi(h); err == nil {
				lastEventID = v
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		countCh, initialCount := subTracker.Join(workflowID)
		defer subTracker.Leave(workflowID, countCh)

		_, _ = fmt.Fprintf(w, "event: clients\ndata: {\"count\": %d}\n\n", initialCount)
		flusher.Flush()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		sc := workflowstreams.NewClient(tc, workflowID, workflowstreams.Options{})
		dc := converter.GetDefaultDataConverter()
		var writeMu sync.Mutex

		go func() {
			keepalive := time.NewTicker(15 * time.Second)
			defer keepalive.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case c := <-countCh:
					writeMu.Lock()
					_, _ = fmt.Fprintf(w, "event: clients\ndata: {\"count\": %d}\n\n", c)
					flusher.Flush()
					writeMu.Unlock()
				case <-keepalive.C:
					writeMu.Lock()
					_, _ = fmt.Fprintf(w, ": keepalive\n\n")
					flusher.Flush()
					writeMu.Unlock()
				}
			}
		}()

		nextOffset := int64(lastEventID + 1)
		for item, err := range sc.Subscribe(ctx, workflowstreams.SubscribeOptions{
			Topics:       []string{topic},
			FromOffset:   nextOffset,
			PollCooldown: 200 * time.Millisecond,
		}) {
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("SSE stream error for %s/%s: %v", workflowID, topic, err)
				}
				return
			}
			if item.Data != nil {
				var val any
				if err := dc.FromPayload(item.Data, &val); err == nil {
					b, _ := json.Marshal(val)
					writeMu.Lock()
					_, _ = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", item.Offset, string(b))
					flusher.Flush()
					writeMu.Unlock()
					if item.Offset+1 > nextOffset {
						nextOffset = item.Offset + 1
					}
				}
			}
		}

		if ctx.Err() == nil {
			if qv, qerr := tc.QueryWorkflow(ctx, workflowID, "", a2ui.StreamStateQueryName); qerr == nil {
				var st workflowstreams.WorkflowStreamState
				if qv.Get(&st) == nil {
					for i, wi := range st.Log {
						globalOffset := st.BaseOffset + int64(i)
						if globalOffset < nextOffset || wi.Topic != topic {
							continue
						}
						if val, derr := a2ui.DecodeWireItem(wi.Data, dc); derr == nil {
							b, _ := json.Marshal(val)
							writeMu.Lock()
							_, _ = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", globalOffset, string(b))
							flusher.Flush()
							writeMu.Unlock()
							nextOffset = globalOffset + 1
						}
					}
				}
			}
			writeMu.Lock()
			_, _ = fmt.Fprintf(w, "event: done\ndata: {}\n\n")
			flusher.Flush()
			writeMu.Unlock()
		}
	}
}
