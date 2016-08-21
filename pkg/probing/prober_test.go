package probing

import (
	"net/http/httptest"
	"testing"
	"time"
)

func Test_Probe_NewHTTPHealthHandler(t *testing.T) {
	s := httptest.NewServer(NewHTTPHealthHandler())

	p := NewProber(nil)
	p.AddHTTP("test-id", time.Millisecond, []string{s.URL})
	defer p.Remove("test-id")

	time.Sleep(100 * time.Millisecond)

	status, err := p.Status("test-id")
	if err != nil {
		t.Fatalf("err = %v, want %v", err, nil)
	}
	if total := status.Total(); total < 50 || total > 150 {
		t.Fatalf("total = %v, want around %v", total, 100)
	}
	if health := status.Health(); !health {
		t.Fatalf("health = %v, want %v", health, true)
	}

	// become unhealthy
	s.Close()

	time.Sleep(100 * time.Millisecond)

	if total := status.Total(); total < 150 || total > 250 {
		t.Fatalf("total = %v, want around %v", total, 200)
	}
	if loss := status.Loss(); loss < 50 || loss > 150 {
		t.Fatalf("loss = %v, want around %v", loss, 200)
	}
	if health := status.Health(); health {
		t.Fatalf("health = %v, want %v", health, false)
	}
}

func Test_Probe_Reset(t *testing.T) {
	s := httptest.NewServer(NewHTTPHealthHandler())
	defer s.Close()

	p := NewProber(nil)
	p.AddHTTP("test-id", time.Millisecond, []string{s.URL})
	defer p.Remove("test-id")

	time.Sleep(100 * time.Millisecond)

	status, err := p.Status("test-id")
	if err != nil {
		t.Fatalf("err = %v, want %v", err, nil)
	}
	if total := status.Total(); total < 50 || total > 150 {
		t.Fatalf("total = %v, want around %v", total, 100)
	}
	if health := status.Health(); !health {
		t.Fatalf("health = %v, want %v", health, true)
	}

	p.Reset("test-id")

	time.Sleep(100 * time.Millisecond)

	if total := status.Total(); total < 50 || total > 150 {
		t.Fatalf("total = %v, want around %v", total, 100)
	}
	if health := status.Health(); !health {
		t.Fatalf("health = %v, want %v", health, true)
	}
}

func Test_Probe_Remove(t *testing.T) {
	s := httptest.NewServer(NewHTTPHealthHandler())
	defer s.Close()

	p := NewProber(nil)
	p.AddHTTP("test-id", time.Millisecond, []string{s.URL})

	p.Remove("test-id")

	if _, err := p.Status("test-id"); err != ErrNotFound {
		t.Fatalf("err = %v, want %v", err, ErrNotFound)
	}
}
