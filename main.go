package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shawnlin0125/plugin-hub/internal/discovery"
	"github.com/shawnlin0125/plugin-hub/internal/handler"
	"github.com/shawnlin0125/plugin-hub/internal/registry"
)

func main() {
	port := flag.Int("port", 8000, "HTTP listen port")
	manifestURL := flag.String("manifest-url", "", "URL to plugin-manifest.json")
	flag.Parse()

	if *manifestURL == "" {
		*manifestURL = os.Getenv("MANIFEST_URL")
	}
	if *manifestURL == "" {
		*manifestURL = "https://raw.githubusercontent.com/shawnlin0125/ticket-vendor/main/plugin-manifest.json"
	}

	// Initialize registry
	reg := registry.New()

	// Fetch plugin manifest (from ticket-vendor repo)
	fetcher := discovery.NewFetcher(*manifestURL)
	manifest, err := fetcher.Fetch()
	if err != nil {
		log.Printf("⚠️  Could not fetch manifest: %v — starting with empty registry", err)
	} else {
		for _, p := range manifest.Plugins {
			reg.LoadFromManifest(p.ID, p.Name, p.Version, p.Description, p.Repo, p.TestPassed)
			testStatus := "⏳"
			if p.TestPassed {
				testStatus = "✅"
			}
			log.Printf("   📦 %s v%s — %s (test: %s)", p.ID, p.Version, p.Description, testStatus)
		}
	}

	// Periodic manifest refresh
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			m, err := fetcher.Fetch()
			if err != nil {
				log.Printf("⚠️  Manifest refresh failed: %v", err)
				continue
			}
			for _, p := range m.Plugins {
				reg.LoadFromManifest(p.ID, p.Name, p.Version, p.Description, p.Repo, p.TestPassed)
			}
			log.Printf("🔄 Manifest refreshed: %d plugins", len(m.Plugins))
		}
	}()

	// Wire up HTTP handlers
	h := handler.New(reg)
	mux := http.NewServeMux()

	// ── Admin API ──
	mux.HandleFunc("GET /", h.Dashboard)
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /api/plugins", h.ListPlugins)
	mux.HandleFunc("GET /api/plugins/{id}", h.GetPlugin)
	mux.HandleFunc("POST /api/plugins/{id}/enable", h.EnablePlugin)
	mux.HandleFunc("POST /api/plugins/{id}/disable", h.DisablePlugin)

	// ── Assignment API ──
	mux.HandleFunc("GET /api/assignments", h.GetAssignments)
	mux.HandleFunc("POST /api/assignments/{vendor}", h.SetAssignment)

	// ── Business Proxy (reverse proxy to ticket-proxy pods) ──
	mux.HandleFunc("GET /api/v1/{vendor}/search", h.ProxyToVendor)
	mux.HandleFunc("POST /api/v1/{vendor}/orders", h.ProxyToVendor)
	mux.HandleFunc("GET /api/v1/{vendor}/orders/{id}", h.ProxyToVendor)
	mux.HandleFunc("GET /api/v1/{vendor}/orders/{id}/poll", h.ProxyToVendor)
	mux.HandleFunc("GET /api/v1/{vendor}/inventory", h.ProxyToVendor)

	corsHandler := corsMiddleware(mux)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("\n🛑 Received %v, shutting down...", sig)
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("🚀 Plugin Hub listening on http://0.0.0.0%s", addr)
	log.Printf("   Dashboard:  http://localhost%s", addr)
	log.Printf("   Admin API:  http://localhost%s/api/plugins", addr)
	log.Printf("   Proxy API:  http://localhost%s/api/v1/{vendor}/*", addr)
	log.Printf("   Manifest:   %s", *manifestURL)

	if err := http.ListenAndServe(addr, corsHandler); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(200)
			return
		}
		next.ServeHTTP(w, r)
	})
}
