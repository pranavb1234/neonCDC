package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		panic("WEBHOOK_SECRET is required")
	}

	mux := http.NewServeMux()
	receiver := receiver{secret: secret, output: os.Stdout}
	mux.HandleFunc("POST /webhook", receiver.receive)

	fmt.Println("Sample receiver listening on http://localhost:8080/webhook")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}

type receiver struct {
	secret string
	output io.Writer
}

func (receiver receiver) receive(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read request", http.StatusBadRequest)
		return
	}
	if !validSignature(receiver.secret, r.Header.Get("X-CDC-Timestamp"), r.Header.Get("X-CDC-Signature"), body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var event struct {
		ID        string `json:"id"`
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(receiver.output, "event_id=%s operation=%s\n", event.ID, event.Operation)
	w.WriteHeader(http.StatusOK)
}

func validSignature(secret, timestamp, signature string, body []byte) bool {
	if timestamp == "" || !strings.HasPrefix(signature, "v1=") {
		return false
	}
	expected, err := hex.DecodeString(sign(secret, timestamp, body))
	if err != nil {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "v1="))
	if err != nil {
		return false
	}
	return hmac.Equal(expected, provided)
}

func sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
