package gui

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerRendersAndHandlesAction(t *testing.T) {
	t.Parallel()

	srv, err := New(ViewModel{
		Title:    "i2tor",
		Subtitle: "status",
		Fields:   map[string]string{"Tor": "ready"},
	}, map[string]func(context.Context) (string, error){
		"install": func(context.Context) (string, error) {
			return "install completed", nil
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	srv.handleIndex(resp, req)
	body, _ := io.ReadAll(resp.Result().Body)
	if !strings.Contains(string(body), "i2tor") {
		t.Fatalf("GET / body missing title: %s", string(body))
	}

	req = httptest.NewRequest("POST", "/action", strings.NewReader("name=install"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp = httptest.NewRecorder()
	srv.handleAction(resp, req)

	req = httptest.NewRequest("GET", "/", nil)
	resp = httptest.NewRecorder()
	srv.handleIndex(resp, req)
	body, _ = io.ReadAll(resp.Result().Body)
	if !strings.Contains(string(body), "install completed") {
		t.Fatalf("GET / body missing action result: %s", string(body))
	}
}
