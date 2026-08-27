// Command tsdb-server runs the TSDB engine as a small HTTP service. It exposes
// write, query, downsample and retention endpoints over HTTP and is the only
// production entry point of the project.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dyl-01/tsdb/internal/clock"
	"github.com/dyl-01/tsdb/internal/config"
	"github.com/dyl-01/tsdb/internal/healthz"
	"github.com/dyl-01/tsdb/internal/query"
	"github.com/dyl-01/tsdb/internal/store"
)

type server struct {
	store  *store.Store
	health *healthz.Prober
}

func main() {
	cfg := config.Default()
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataDir := flag.String("data-dir", cfg.DataDir, "engine data directory")
	flag.Int64Var(&cfg.ShardInterval, "shard-interval", cfg.ShardInterval, "shard interval (ms)")
	flag.Int64Var(&cfg.Retention, "retention", cfg.Retention, "retention window (ms)")
	flag.Int64Var(&cfg.DownsampleInterval, "downsample-interval", cfg.DownsampleInterval, "downsample bucket (ms)")
	flag.IntVar(&cfg.MaxMemPoints, "max-mem-points", cfg.MaxMemPoints, "memtable flush threshold")
	flag.Parse()

	eng, err := store.New(cfg, clock.NewReal(), *dataDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer eng.Close()

	s := &server{store: eng, health: healthz.New(eng)}
	mux := http.NewServeMux()
	mux.HandleFunc("/write", s.handleWrite)
	mux.HandleFunc("/write-batch", s.handleWriteBatch)
	mux.HandleFunc("/query", s.handleQuery)
	mux.HandleFunc("/query-all", s.handleQueryAll)
	mux.HandleFunc("/query-label", s.handleQueryLabel)
	mux.HandleFunc("/downsample", s.handleDownsample)
	mux.HandleFunc("/downsample-series", s.handleDownsampleSeries)
	mux.HandleFunc("/series", s.handleSeries)
	mux.HandleFunc("/retention", s.handleRetention)
	mux.HandleFunc("/compact", s.handleCompact)
	mux.HandleFunc("/flush", s.handleFlush)
	mux.HandleFunc("/seal", s.handleSeal)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/healthz/ready", s.handleReady)
	mux.HandleFunc("/healthz/history", s.handleHealthHistory)

	log.Printf("config: %s", cfg.Describe())
	log.Printf("tsdb-server listening on %s (data=%s)", *addr, *dataDir)

	httpServer := &http.Server{Addr: *addr, Handler: withLogging(mux)}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Printf("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

type writeRequest struct {
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels"`
	Timestamp int64             `json:"timestamp"`
	Value     float64           `json:"value"`
}

func (s *server) handleWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req writeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.store.Write(req.Name, req.Labels, req.Timestamp, req.Value); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleQuery(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	labels := parseLabels(r)
	start, err := strconv.ParseInt(r.URL.Query().Get("start"), 10, 64)
	if err != nil {
		http.Error(w, "invalid start", http.StatusBadRequest)
		return
	}
	end, err := strconv.ParseInt(r.URL.Query().Get("end"), 10, 64)
	if err != nil {
		http.Error(w, "invalid end", http.StatusBadRequest)
		return
	}
	points, err := s.store.Query(name, labels, start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if lim := r.URL.Query().Get("limit"); lim != "" {
		if n, err := strconv.Atoi(lim); err == nil {
			points = query.Limit(points, n)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "points": points})
}

// parseLabels extracts repeated label=k=v query parameters into a map.
func parseLabels(r *http.Request) map[string]string {
	values, ok := r.URL.Query()["label"]
	if !ok || len(values) == 0 {
		return nil
	}
	labels := make(map[string]string)
	for _, v := range values {
		if eq := strings.IndexByte(v, '='); eq > 0 {
			labels[v[:eq]] = v[eq+1:]
		}
	}
	if len(labels) == 0 {
		return nil
	}
	return labels
}

func (s *server) handleQueryAll(w http.ResponseWriter, r *http.Request) {
	start, err := strconv.ParseInt(r.URL.Query().Get("start"), 10, 64)
	if err != nil {
		http.Error(w, "invalid start", http.StatusBadRequest)
		return
	}
	end, err := strconv.ParseInt(r.URL.Query().Get("end"), 10, 64)
	if err != nil {
		http.Error(w, "invalid end", http.StatusBadRequest)
		return
	}
	points, err := s.store.QueryAll(start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"points": points})
}

func (s *server) handleDownsample(w http.ResponseWriter, r *http.Request) {
	ts, err := strconv.ParseInt(r.URL.Query().Get("ts"), 10, 64)
	if err != nil {
		http.Error(w, "invalid ts", http.StatusBadRequest)
		return
	}
	agg := r.URL.Query().Get("agg")
	if agg == "" {
		agg = "avg"
	}
	points, err := s.store.DownsampleWithAgg(ts, agg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"points": points})
}

func (s *server) handleDownsampleSeries(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	ts, err := strconv.ParseInt(r.URL.Query().Get("ts"), 10, 64)
	if err != nil {
		http.Error(w, "invalid ts", http.StatusBadRequest)
		return
	}
	points, err := s.store.DownsampleSeries(name, nil, ts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"points": points})
}

func (s *server) handleRetention(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	removed, err := s.store.EnforceRetention()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"removed": removed})
}

type writeBatchRequest struct {
	Points []writeRequest `json:"points"`
}

func (s *server) handleWriteBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req writeBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	batch := make([]store.BatchPoint, 0, len(req.Points))
	for _, p := range req.Points {
		batch = append(batch, store.BatchPoint{
			Name:      p.Name,
			Labels:    p.Labels,
			Timestamp: p.Timestamp,
			Value:     p.Value,
		})
	}
	if err := s.store.WriteBatch(batch); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"written": len(batch)})
}

func (s *server) handleQueryLabel(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("label")
	value := r.URL.Query().Get("value")
	start, err := strconv.ParseInt(r.URL.Query().Get("start"), 10, 64)
	if err != nil {
		http.Error(w, "invalid start", http.StatusBadRequest)
		return
	}
	end, err := strconv.ParseInt(r.URL.Query().Get("end"), 10, 64)
	if err != nil {
		http.Error(w, "invalid end", http.StatusBadRequest)
		return
	}
	points, err := s.store.QueryByLabel(name, value, start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"points": points})
}

func (s *server) handleCompact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ts, err := strconv.ParseInt(r.URL.Query().Get("ts"), 10, 64)
	if err != nil {
		http.Error(w, "invalid ts", http.StatusBadRequest)
		return
	}
	if err := s.store.CompactShard(ts); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleFlush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ts, err := strconv.ParseInt(r.URL.Query().Get("ts"), 10, 64)
	if err != nil {
		http.Error(w, "invalid ts", http.StatusBadRequest)
		return
	}
	if err := s.store.FlushShard(ts); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleSeal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ts, err := strconv.ParseInt(r.URL.Query().Get("ts"), 10, 64)
	if err != nil {
		http.Error(w, "invalid ts", http.StatusBadRequest)
		return
	}
	if err := s.store.SealShard(ts); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Stats())
}

func (s *server) handleSeries(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"series": s.store.ListSeries()})
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	if s.health == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"data_dir": s.store.DataDir(),
			"shards":   s.store.ShardInfos(),
			"stats":    s.store.Stats(),
		})
		return
	}
	writeJSON(w, http.StatusOK, s.health.Check(context.Background()))
}

func (s *server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.health == nil || !s.health.Ready(r.Context()) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *server) handleHealthHistory(w http.ResponseWriter, _ *http.Request) {
	if s.health == nil {
		writeJSON(w, http.StatusOK, map[string]any{"recent": []any{}, "summary": map[string]any{}})
		return
	}
	recent := s.health.Recent(1)
	alarm := "none"
	if len(recent) == 1 {
		alarm = healthz.AlarmSeverity(healthz.Validate(recent[0]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"recent":  s.health.Recent(10),
		"summary": s.health.Summary(),
		"trend":   s.health.Trend(10),
		"alarm":   alarm,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// withLogging wraps a handler with a request log line.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
