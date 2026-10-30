package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchURLRefusesLocalAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("secret")) }))
	defer srv.Close()
	if _, err := FetchURL(context.Background(), srv.URL); err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("FetchURL(%s) = %v, want a non-public refusal", srv.URL, err)
	}
}

func TestPublicOnly(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:80": false, "[::1]:80": false, "10.1.2.3:80": false, "192.168.0.1:80": false,
		"169.254.169.254:80": false, "100.64.0.1:80": false, "[fe80::1]:80": false, "0.0.0.0:80": false,
		"[::ffff:127.0.0.1]:80": false, "93.184.216.34:443": true, "[2606:4700::1]:443": true,
	} {
		if err := publicOnly("tcp", addr, nil); (err == nil) != ok {
			t.Errorf("publicOnly(%s) = %v, want ok=%v", addr, err, ok)
		}
	}
}
