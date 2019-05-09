package server

import (
	"context"
	"gitlab.com/pennersr/shove/internal/queue"
	"gitlab.com/pennersr/shove/internal/services"
	"log"
	"net/http"
	"sync"
)

type Server struct {
	server       *http.Server
	shuttingDown bool
	queueFactory queue.QueueFactory
	workers      map[string]*worker
	feedbackLock sync.Mutex
	feedback     []tokenFeedback
}

func NewServer(addr string, qf queue.QueueFactory) (s *Server) {
	mux := http.NewServeMux()

	h := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	s = &Server{
		server:       h,
		queueFactory: qf,
		workers:      make(map[string]*worker),
		feedback:     make([]tokenFeedback, 0),
	}
	mux.HandleFunc("/api/push", s.handlePush)
	mux.HandleFunc("/api/feedback", s.handleFeedback)
	return s
}

func (s *Server) Serve() (err error) {
	log.Println("Shove server started")
	err = s.server.ListenAndServe()
	if s.shuttingDown {
		err = nil
	}
	return
}

func (s *Server) Shutdown(ctx context.Context) (err error) {
	s.shuttingDown = true
	s.server.Shutdown(ctx)
	if err = s.server.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down Shove server: %v\n", err)
		return
	}
	log.Println("Shove server stopped")
	for _, w := range s.workers {
		err = w.shutdown()
		if err != nil {
			return
		}
	}
	return
}

func (s *Server) AddService(pp services.PushService) (err error) {
	log.Printf("Initializing %s service", pp)
	q, err := s.queueFactory.NewQueue(pp.ID())
	if err != nil {
		return
	}
	w, err := newWorker(pp, q)
	if err != nil {
		return
	}
	go w.serve(s)
	s.workers[pp.ID()] = w
	return
}