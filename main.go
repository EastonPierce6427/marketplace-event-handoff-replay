package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	s := &Server{api: &Client{key: key, http: &http.Client{}}, ledger: newLedger()}
	mux := http.NewServeMux()
	mux.HandleFunc("/replay", s.replay)
	mux.HandleFunc("/redrive", s.redrive)
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Fatal(http.ListenAndServe(addr, mux))
}
