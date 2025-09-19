package apiserver

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/VitalyCone/kaspersky-container-security-task/internal/config"
	"github.com/VitalyCone/kaspersky-container-security-task/internal/domain/dto"
	wp "github.com/VitalyCone/kaspersky-container-security-task/internal/workerpool"
)

type APIServer struct{
	srv *http.Server
	wp *wp.WorkerPool
	config *config.WorkerConfig
}

func New(port string, workerPool *wp.WorkerPool, config *config.WorkerConfig) *APIServer {
	return &APIServer{
		srv: &http.Server{
			Addr: ":" + port,
		},
		wp: workerPool,
		config: config,
	}
}

func (s *APIServer) Run() error {
	s.wp.Start(s.config.WorkersCount)

	http.HandleFunc("/enqueue", s.handleEnqueue)
	http.HandleFunc("/healthz", s.healthzHandler)

	log.Printf("Server starting on port %s...\n", s.srv.Addr)
	return http.ListenAndServe(s.srv.Addr, nil)
}

func (a *APIServer) Stop(ctx context.Context) error {
	log.Println("worker poll stopping...")
	if err := a.wp.Stop(); err != nil {
		log.Println("error while stopping worker pool: ", err)
	}
	log.Println("worker pool stopped")
	return a.srv.Shutdown(ctx)
}

func (s *APIServer) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var task dto.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if task.ID == "" || task.Payload == "" || task.MaxRetries < 0 {
		http.Error(w, "Invalid task", http.StatusBadRequest)
		return
	}
	if err := s.wp.AddTask(&wp.Task{Task: task}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return 
	}

	w.WriteHeader(http.StatusOK)
}

func (s *APIServer) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}