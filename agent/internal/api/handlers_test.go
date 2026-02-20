package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/driversti/hola/internal/api"
	"github.com/driversti/hola/internal/auth"
)

func newTestRouter() http.Handler {
	return api.NewRouter("0.1.0-test", auth.NewMiddleware("test-token"), nil)
}

func TestHealthEndpoint(t *testing.T) {
	srv := httptest.NewServer(newTestRouter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("want status ok, got %q", body["status"])
	}
}

func TestAgentInfoRequiresAuth(t *testing.T) {
	srv := httptest.NewServer(newTestRouter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/agent/info")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestAgentInfoWithAuth(t *testing.T) {
	srv := httptest.NewServer(newTestRouter())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/agent/info", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var info struct {
		Version  string `json:"version"`
		Hostname string `json:"hostname"`
		OS       string `json:"os"`
		Arch     string `json:"arch"`
	}
	json.NewDecoder(resp.Body).Decode(&info)

	if info.Version != "0.1.0-test" {
		t.Errorf("want version 0.1.0-test, got %q", info.Version)
	}
	if info.OS != runtime.GOOS {
		t.Errorf("want OS %s, got %q", runtime.GOOS, info.OS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("want arch %s, got %q", runtime.GOARCH, info.Arch)
	}
}
