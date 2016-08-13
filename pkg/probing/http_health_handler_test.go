package probing

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func Test_NewHTTPHealthHandler(t *testing.T) {
	hd := NewHTTPHealthHandler()

	recorder := httptest.NewRecorder()
	hd.ServeHTTP(recorder, nil)
	recorder.Flush()

	body := recorder.Body.String()

	var health Health
	err := json.NewDecoder(strings.NewReader(body)).Decode(&health)
	if err != nil {
		t.Fatal(err)
	}
	if !health.OK {
		t.Fatalf("expected true, got %+v", health)
	}
	if time.Now().Before(health.RequestedTime) {
		t.Fatalf("unexpected %+v", health)
	}
}
