package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/shawnlin0125/plugin-hub/internal/config"
	"github.com/shawnlin0125/plugin-hub/internal/handler"
	"github.com/shawnlin0125/plugin-hub/internal/registry"
)

func main() {
	port := flag.Int("port", 8000, "HTTP listen port")
	configPath := flag.String("config", "", "Path to plugins.yaml")
	flag.Parse()

	// Load plugin config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("📄 Loaded config with %d plugin(s)", len(cfg.Plugins))

	// Initialize registry
	reg := registry.New()
	for _, p := range cfg.Plugins {
		route := os.Getenv("ROUTE_" + p.ID)
		if route == "" {
			route = "default"
		}
		reg.Load(p.ID, p.Name, p.Version, p.Description, p.Repo, route)
		log.Printf("   📦 %s v%s — %s (route: %s)", p.ID, p.Version, p.Description, route)
	}

	// Wire up HTTP handlers
	h := handler.New(reg)
	mux := http.NewServeMux()

	// Dashboard
	mux.HandleFunc("GET /", h.Dashboard)

	// Health
	mux.HandleFunc("GET /health", h.Health)

	// Plugin CRUD
	mux.HandleFunc("GET /api/plugins", h.ListPlugins)
	mux.HandleFunc("GET /api/plugins/{id}", h.GetPlugin)
	mux.HandleFunc("POST /api/plugins/{id}/enable", h.EnablePlugin)
	mux.HandleFunc("POST /api/plugins/{id}/disable", h.DisablePlugin)
	mux.HandleFunc("POST /api/plugins/{id}/test", h.TestPlugin)

	// CORS middleware for SPA
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
	log.Printf("   Dashboard: http://localhost%s", addr)
	log.Printf("   API:       http://localhost%s/api/plugins", addr)

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
