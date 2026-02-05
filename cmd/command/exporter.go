/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package command

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewExporterCmd() *cobra.Command {
	var socketPath, listenAddr string
	exporterCmd := &cobra.Command{
		Use:   "exporter",
		Short: "Run metrics exporter (proxy TCP to sichek metrics Unix socket)",
		Long:  "Listen on a TCP address and proxy GET /metrics to the sichek daemon metrics Unix socket. Run sichek daemon with --metrics-socket first, then run sichek exporter so Prometheus can scrape via TCP.",
		Run: func(cmd *cobra.Command, args []string) {
			if socketPath == "" {
				logrus.WithField("component", "exporter").Errorf("metrics socket is not set")
				os.Exit(1)
			}
			if listenAddr == "" {
				logrus.WithField("component", "exporter").Errorf("listen address is not set")
				os.Exit(1)
			}

			client := &http.Client{
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
						return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
					},
					DisableKeepAlives: true,
				},
				Timeout: 5 * time.Second,
			}

			copyHeader := func(dst http.ResponseWriter, src http.Header) {
				for _, k := range []string{"Content-Type", "Content-Encoding"} {
					if v := src.Get(k); v != "" {
						dst.Header().Set(k, v)
					}
				}
			}

			mux := http.NewServeMux()
			mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					http.Error(w, "method not allowed\n", http.StatusMethodNotAllowed)
					return
				}
				req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://unix/metrics", nil)
				if err != nil {
					http.Error(w, "bad request\n", http.StatusBadRequest)
					return
				}
				resp, err := client.Do(req)
				if err != nil {
					http.Error(w, "sichek metrics unavailable\n", http.StatusServiceUnavailable)
					logrus.WithField("component", "exporter").Warnf("proxy failed: %v", err)
					return
				}
				defer resp.Body.Close()
				copyHeader(w, resp.Header)
				w.WriteHeader(resp.StatusCode)
				if _, err := io.Copy(w, resp.Body); err != nil && err != io.EOF {
					logrus.WithField("component", "exporter").Debugf("write response: %v", err)
				}
			})
			mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
				if err != nil {
					http.Error(w, "sichek metrics socket unavailable\n", http.StatusServiceUnavailable)
					return
				}
				conn.Close()
				w.WriteHeader(http.StatusOK)
			})

			srv := &http.Server{Addr: listenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
			go func() {
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
				<-sigCh
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := srv.Shutdown(ctx); err != nil {
					logrus.WithField("component", "exporter").Warnf("shutdown: %v", err)
				}
			}()

			logrus.WithField("component", "exporter").Infof("listening=%s socket=%s", listenAddr, socketPath)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logrus.WithField("component", "exporter").Errorf("exporter failed: %v", err)
				os.Exit(1)
			}
		},
	}
	exporterCmd.Flags().StringVar(&socketPath, "metrics-socket", "/var/run/sichek/metrics.sock", "Path to sichek metrics Unix socket")
	exporterCmd.Flags().StringVar(&listenAddr, "listen", ":19091", "TCP address to listen on")
	return exporterCmd
}
