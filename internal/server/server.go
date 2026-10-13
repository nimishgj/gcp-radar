package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nimishgj/gcp-radar/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

type Server struct {
	httpServer *http.Server
	config     *config.ServerConfig
}

func New(cfg *config.ServerConfig) *Server {
	mux := http.NewServeMux()

	mux.Handle(cfg.MetricsPath, promhttp.Handler())

	mux.HandleFunc(cfg.HealthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>GCP Radar</title></head>
<body>
<h1>GCP Radar</h1>
<p>GCP Resource &amp; Cost Monitoring Agent</p>
<ul>
<li><a href="%s">Metrics</a></li>
<li><a href="%s">Health</a></li>
</ul>
</body>
</html>
`, cfg.MetricsPath, cfg.HealthPath)
	})

	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		config: cfg,
	}
}

func (s *Server) Start() error {
	log.Info().
		Int("port", s.config.Port).
		Str("metrics_path", s.config.MetricsPath).
		Str("health_path", s.config.HealthPath).
		Msg("Starting HTTP server")

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Info().Msg("Shutting down HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// Handler exposes the mux for testing.
func (s *Server) Handler() http.Handler { return s.httpServer.Handler }
