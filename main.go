package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var (
	httpAddr string
	raftAddr string
	nodeID   string
	joinAddr string
	dataDir  string
)

func init() {
	flag.StringVar(&httpAddr, "http", "127.0.0.1:8080", "HTTP server address")
	flag.StringVar(&raftAddr, "raft", "127.0.0.1:7000", "Raft server address")
	flag.StringVar(&nodeID, "id", "", "Node ID. If not set, it will be the same as the Raft address.")
	flag.StringVar(&joinAddr, "join", "", "Address of a node to join")
	flag.StringVar(&dataDir, "data", "data", "Directory to store Raft data")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if nodeID == "" {
		nodeID = raftAddr
	}

	// Ensure data directory exists.
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("failed to create data directory: %s", err)
	}

	// Create the store.
	s, err := NewStore(StoreConfig{
		NodeID:   nodeID,
		RaftAddr: raftAddr,
		RaftDir:  filepath.Join(dataDir, nodeID),
	})
	if err != nil {
		log.Fatalf("failed to create store: %s", err)
	}

	// If joinAddr is set, this node will join an existing cluster.
	// Otherwise, it will bootstrap a new cluster.
	bootstrap := joinAddr == ""
	if err := s.Open(bootstrap, nodeID); err != nil {
		log.Fatalf("failed to open store: %s", err)
	}

	// If a join address is provided, the node will join the existing cluster.
	if joinAddr != "" {
		if err := join(joinAddr, raftAddr, nodeID); err != nil {
			log.Fatalf("failed to join cluster: %s", err)
		}
	}

	// Start the HTTP server.
	h := NewHttpService(s)
	log.Printf("starting HTTP server at %s", httpAddr)
	if err := http.ListenAndServe(httpAddr, h); err != nil {
		log.Fatalf("failed to start HTTP server: %s", err)
	}
}
