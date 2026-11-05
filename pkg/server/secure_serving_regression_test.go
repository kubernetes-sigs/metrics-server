// Copyright 2026 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"sigs.k8s.io/metrics-server/cmd/metrics-server/app/options"
)

// TestAggregatedAPIGroupHeaders exercises the secure serving path used by the
// aggregated API when kube-apiserver forwards a user with many groups.
func TestAggregatedAPIGroupHeaders(t *testing.T) {
	for _, tc := range []struct {
		name         string
		disableHTTP2 bool
		protocol     int
	}{
		{"http1", true, 1}, {"http2", false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			opts := options.NewOptions()
			opts.DisableAuthForTesting = true
			opts.SecureServing.Listener = listener
			opts.SecureServing.DisableHTTP2Serving = tc.disableHTTP2
			opts.SecureServing.ServerCert.CertDirectory = ""
			config, err := opts.ApiserverConfig()
			if err != nil {
				t.Fatal(err)
			}
			serving := config.SecureServing
			certPEM, _ := serving.Cert.CurrentCertKeyContent()
			roots := x509.NewCertPool()
			if !roots.AppendCertsFromPEM(certPEM) {
				t.Fatal("failed to trust test certificate")
			}
			stop := make(chan struct{})
			stopped, listenerStopped, err := serving.Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Group-Count", strconv.Itoa(len(r.Header.Values("X-Remote-Group"))))
				w.WriteHeader(http.StatusOK)
			}), 5*time.Second, stop)
			if err != nil {
				listener.Close()
				t.Fatal(err)
			}
			t.Cleanup(func() { close(stop); <-listenerStopped; <-stopped })
			transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}, ForceAttemptHTTP2: !tc.disableHTTP2}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
			req, err := http.NewRequest(http.MethodGet, "https://"+listener.Addr().String()+"/livez", nil)
			if err != nil {
				t.Fatal(err)
			}
			const groups = 600
			for i := range groups {
				req.Header.Add("X-Remote-Group", fmt.Sprintf("group-%d", i))
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.ProtoMajor != tc.protocol {
				t.Fatalf("protocol = %s, want HTTP/%d", resp.Proto, tc.protocol)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if got := resp.Header.Get("Group-Count"); got != strconv.Itoa(groups) {
				t.Errorf("handler received %s groups, want %d", got, groups)
			}
		})
	}
}
