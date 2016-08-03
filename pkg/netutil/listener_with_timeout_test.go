package netutil

import (
	"net"
	"testing"
	"time"
)

// (etcd pkg.transport.TestNewTimeoutListener)
func Test_NewListenerWithTimeout(t *testing.T) {
	l, err := NewListenerWithTimeout("127.0.0.1:0", "http", nil, time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("unexpected NewListenerWithTimeout error: %v", err)
	}
	defer l.Close()

	tln := l.(*listenerWithTimeout)
	if tln.writeTimeout != time.Hour {
		t.Errorf("write timeout = %s, want %s", tln.writeTimeout, time.Hour)
	}
	if tln.readTimeout != time.Hour {
		t.Errorf("read timeout = %s, want %s", tln.readTimeout, time.Hour)
	}
}

// (etcd pkg.transport.TestWriteReadTimeoutListener)
func Test_ListenerWithTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unexpected listen error: %v", err)
	}
	wln := listenerWithTimeout{
		Listener:     ln,
		writeTimeout: 10 * time.Millisecond,
		readTimeout:  10 * time.Millisecond,
	}
	stop := make(chan struct{})

	blocker := func() {
		conn, derr := net.Dial("tcp", ln.Addr().String())
		if derr != nil {
			t.Fatalf("unexpected dail error: %v", derr)
		}
		defer conn.Close()
		// block the receiver until the writer timeout
		<-stop
	}
	go blocker()

	conn, err := wln.Accept()
	if err != nil {
		t.Fatalf("unexpected accept error: %v", err)
	}
	defer conn.Close()

	// fill the socket buffer
	data := make([]byte, 5*1024*1024)
	done := make(chan struct{})
	go func() {
		_, err = conn.Write(data)
		done <- struct{}{}
	}()

	select {
	case <-done:
	// It waits 1s more to avoid delay in low-end system.
	case <-time.After(wln.writeTimeout*10 + time.Second):
		t.Fatal("wait timeout")
	}

	if operr, ok := err.(*net.OpError); !ok || operr.Op != "write" || !operr.Timeout() {
		t.Errorf("err = %v, want write i/o timeout error", err)
	}
	stop <- struct{}{}

	go blocker()

	conn, err = wln.Accept()
	if err != nil {
		t.Fatalf("unexpected accept error: %v", err)
	}
	buf := make([]byte, 10)

	go func() {
		_, err = conn.Read(buf)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(wln.readTimeout * 10):
		t.Fatal("wait timeout")
	}

	if operr, ok := err.(*net.OpError); !ok || operr.Op != "read" || !operr.Timeout() {
		t.Errorf("err = %v, want write i/o timeout error", err)
	}
	stop <- struct{}{}
}
