package main

import (
	"embed"
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"net/http"
	"time"

	"tiered-io/sim"
)

//go:embed web/index.html web/app.css web/app.js
var webFS embed.FS

var pageTpl = template.Must(template.ParseFS(webFS, "web/index.html"))

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	flag.Parse()
	srv := &http.Server{
		Addr:              *addr,
		Handler:           newMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("tiered-io simulator at http://%s", *addr)
	log.Fatal(srv.ListenAndServe())
}

func newMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", index)
	mux.HandleFunc("GET /app.css", static("app.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /app.js", static("app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("POST /api/simulate", simulate)
	return mux
}

type pageData struct {
	Boot template.JS
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	raw, err := json.Marshal(sim.DefaultConfig())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := pageTpl.Execute(w, pageData{Boot: template.JS(raw)}); err != nil {
		log.Printf("template: %v", err)
	}
}

func static(name, contentType string) http.HandlerFunc {
	body, err := webFS.ReadFile("web/" + name)
	return func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	}
}

func simulate(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, 1<<20)
	defer body.Close()
	dec := json.NewDecoder(body)
	var cfg sim.Config
	if err := dec.Decode(&cfg); err != nil {
		http.Error(w, "invalid configuration: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, sim.Evaluate(cfg))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("encode: %v", err)
	}
}
