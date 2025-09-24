package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// HttpService provides an HTTP interface to the key-value store.
type HttpService struct {
	store *Store
	mux   *http.ServeMux
}

// NewHttpService returns a new instance of HttpService.
func NewHttpService(store *Store) *HttpService {
	mux := http.NewServeMux()
	s := &HttpService{
		store: store,
		mux:   mux,
	}
	mux.HandleFunc("/key/", s.handleKeyRequest)
	mux.HandleFunc("/join", s.handleJoin)
	return s
}

func (s *HttpService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *HttpService) handleKeyRequest(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/key/")
	switch r.Method {
	case "GET":
		v, err := s.store.Get(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte(v))
	case "PUT":
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		if s.store.raft.State().String() != "Leader" {
			leader := string(s.store.raft.Leader())
			// Redirect to leader
			// In a real-world scenario, you might want to proxy the request instead.
			// The client would need to handle the redirect.
			http.Redirect(w, r, "http://"+leader+r.URL.Path, http.StatusTemporaryRedirect)
			return
		}

		err = s.store.Set(key, string(body))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *HttpService) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var m map[string]string
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	remoteAddr, ok := m["addr"]
	if !ok {
		http.Error(w, "missing 'addr' in request body", http.StatusBadRequest)
		return
	}
	nodeID, ok := m["id"]
	if !ok {
		http.Error(w, "missing 'id' in request body", http.StatusBadRequest)
		return
	}

	if err := s.store.Join(nodeID, remoteAddr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// join sends a request to a node in the cluster to join it.
func join(joinAddr, raftAddr, nodeID string) error {
	b, err := json.Marshal(map[string]string{"addr": raftAddr, "id": nodeID})
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/join", joinAddr), "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	log.Printf("join request sent to %s, response status: %s", joinAddr, resp.Status)
	return nil
}
