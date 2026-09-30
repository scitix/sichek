package command

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mockUnixServer(t *testing.T, socketPath string, handler func(w http.ResponseWriter, r *http.Request)) (stop func()) {
	os.Remove(socketPath)
	l, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	stopCh := make(chan struct{})
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				select {
				case <-stopCh:
					return
				default:
					t.Logf("accept error: %v", err)
					continue
				}
			}
			go func(c net.Conn) {
				defer c.Close()
				req, err := http.ReadRequest(bufio.NewReader(c))
				if err != nil {
					return
				}
				w := &mockConnWriter{c: c}
				handler(w, req)
				c.Close()
			}(conn)
		}
	}()
	return func() {
		close(stopCh)
		l.Close()
		os.Remove(socketPath)
	}
}

type mockConnWriter struct {
	c             net.Conn
	h             http.Header
	statusCode    int
	headerWritten bool
}

func (w *mockConnWriter) Header() http.Header {
	if w.h == nil {
		w.h = make(http.Header)
	}
	return w.h
}

func (w *mockConnWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *mockConnWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		code := w.statusCode
		if code == 0 {
			code = http.StatusOK
		}
		statusLine := "HTTP/1.1 " + strconv.Itoa(code) + " " + http.StatusText(code) + "\r\n"
		if _, err := w.c.Write([]byte(statusLine)); err != nil {
			return 0, err
		}
		for k, v := range w.h {
			if k == ":status" {
				continue
			}
			for _, vv := range v {
				if _, err := w.c.Write([]byte(k + ": " + vv + "\r\n")); err != nil {
					return 0, err
				}
			}
		}
		if _, err := w.c.Write([]byte("\r\n")); err != nil {
			return 0, err
		}
		w.headerWritten = true
	}
	return w.c.Write(b)
}

func TestExporterMetricsHealthReady(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "metrics.sock")

	stopServer := mockUnixServer(t, socketPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metrics":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(200)
			w.Write([]byte("metric1 123\n"))
		default:
			w.WriteHeader(404)
		}
	})
	defer stopServer()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	ln.Close()

	listenAddr := "127.0.0.1:" + port
	cmd := NewExporterCmd()
	cmd.SetArgs([]string{"--metrics-socket", socketPath, "--listen", listenAddr})

	go func() {
		_ = cmd.Execute()
	}()

	time.Sleep(300 * time.Millisecond)

	url := "http://" + listenAddr

	doRequest := func(path string) (int, string) {
		resp, err := http.Get(url + path)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	status, body := doRequest("/metrics")
	require.Equal(t, 200, status)
	require.Contains(t, body, "metric1 123")

	status, _ = doRequest("/health")
	require.Equal(t, 200, status)
}
