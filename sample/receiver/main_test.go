package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReceiverAcceptsValidSignature(t *testing.T) {
	body := []byte(`{"id":"event-1","operation":"insert"}`)
	request := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	request.Header.Set("X-CDC-Timestamp", "1731690000")
	request.Header.Set("X-CDC-Signature", "v1="+sign("demo-secret", "1731690000", body))
	recorder := httptest.NewRecorder()

	output := &bytes.Buffer{}
	receiver{secret: "demo-secret", output: output}.receive(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if output.String() != "event_id=event-1 operation=insert\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestReceiverRejectsModifiedBody(t *testing.T) {
	original := []byte(`{"id":"event-1","operation":"insert"}`)
	modified := []byte(`{"id":"event-1","operation":"delete"}`)
	request := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(modified))
	request.Header.Set("X-CDC-Timestamp", "1731690000")
	request.Header.Set("X-CDC-Signature", "v1="+sign("demo-secret", "1731690000", original))
	recorder := httptest.NewRecorder()

	receiver{secret: "demo-secret", output: &bytes.Buffer{}}.receive(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}
