package main

//go:generate bash ./gen.sh

import (
	"log"
	"net/http"
	"sync"
)

type server struct {
	cfg *Config
	mx  *sync.Mutex
}

func (s *server) serveIndex(w http.ResponseWriter, req *http.Request) {
	s.mx.Lock()
	cpy := *s.cfg
	s.mx.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	err := tmpl.Execute(w, cpy)
	if err != nil {
		log.Fatal("ERROR: ", err)
	}
}
func (s *server) serveSave(w http.ResponseWriter, req *http.Request) {

	http.Redirect(w, req, "/", 302)
}
