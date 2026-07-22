package cmd

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func Test_validateMetricsPort(t *testing.T) {
	tests := []struct {
		name        string
		apiPort     string
		metricsPort string
		wantErr     bool
	}{
		{name: "disabled when empty", apiPort: "8314", metricsPort: "", wantErr: false},
		{name: "distinct ports ok", apiPort: "8314", metricsPort: "9090", wantErr: false},
		{name: "same port rejected", apiPort: "8314", metricsPort: "8314", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMetricsPort(tt.apiPort, tt.metricsPort)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func Test_metricsServerExposesMetrics(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv := newMetricsServer(addr)
	go func() {
		_ = srv.ListenAndServe()
	}()
	defer func() {
		_ = srv.Close()
	}()

	var body string
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get("http://" + addr + "/metrics")
		if err == nil {
			b, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				t.Fatalf("read body: %v", readErr)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			body = string(b)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("metrics server not ready: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	for _, want := range []string{
		"flockman_build_info",
		"flockman_registered_services",
		"go_goroutines",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics body missing %q", want)
		}
	}
}

func Test_recordServiceStatusIncrementsCounter(t *testing.T) {
	initMetrics()
	before := gatherCounter(t, "flockman_service_status_requests_total", "result", "bad_request")
	recordServiceStatus("bad_request")
	after := gatherCounter(t, "flockman_service_status_requests_total", "result", "bad_request")
	if after != before+1 {
		t.Fatalf("counter = %v, want %v", after, before+1)
	}
}

func gatherCounter(t *testing.T, name, label, value string) float64 {
	t.Helper()
	families, err := metricsRegistry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == label && l.GetValue() == value {
					return m.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}
