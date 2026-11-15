package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", receive)

	fmt.Println("Sample receiver listening on http://localhost:8080/events")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}

func receive(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read request", http.StatusBadRequest)
		return
	}

	var event any
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	pretty, _ := json.MarshalIndent(event, "", "  ")
	fmt.Println(string(pretty))
	w.WriteHeader(http.StatusNoContent)
}
