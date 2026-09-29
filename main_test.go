package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tiered-io/sim"
)

func TestSimulateEndpoint(t *testing.T) {
	mux := newMux()
	payload, err := json.Marshal(sim.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/simulate", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var got sim.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Tape.BandwidthGBps != 300 {
		t.Fatalf("tape bandwidth %g", got.Tape.BandwidthGBps)
	}
	if got.HDD.Binding != sim.BindStream {
		t.Fatalf("binding %s", got.HDD.Binding)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("index status %d", rec.Code)
	}
	page := rec.Body.String()
	if !strings.Contains(page, "window.BOOT") || !strings.Contains(page, "BW(n)") {
		t.Fatal("index missing boot config or formula")
	}

	req = httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "readConfig") {
		t.Fatalf("app.js status %d", rec.Code)
	}
}
