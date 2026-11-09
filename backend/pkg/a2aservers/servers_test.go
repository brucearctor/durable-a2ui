package a2aservers_test

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ab/durable-a2ui/backend/pkg/a2aservers"
)

func TestA2AJSONRPCServersEndToEnd(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	mux := a2aservers.RestaurantFinderServer(port)
	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(ln)
	}()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reply, err := a2aservers.CallA2AAgent(ctx, "http://"+ln.Addr().String(), "Recommend tacos in CDMX")
	if err != nil {
		t.Fatalf("CallA2AAgent failed: %v", err)
	}
	if !strings.Contains(reply, "Taquería Orinoco CDMX") {
		t.Fatalf("expected Taquería Orinoco CDMX in A2A response, got: %s", reply)
	}
}
