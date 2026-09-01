package valhalla

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDistanceSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != sourcesToTargetsEndpoint {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"sources_to_targets": [[{"distance": 12.5, "time": 700, "from_index": 0, "to_index": 0}]],
			"units": "kilometers"
		}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	result, err := client.Distance(
		context.Background(),
		Location{Lat: 56.064384, Lon: 37.813248},
		Location{Lat: 56.39521, Lon: 37.52663},
		"auto",
	)
	if err != nil {
		t.Fatalf("Distance: %v", err)
	}
	if result.DistanceKM != 12.5 {
		t.Fatalf("unexpected distance: %v", result.DistanceKM)
	}
	if result.TimeSeconds != 700 {
		t.Fatalf("unexpected time: %v", result.TimeSeconds)
	}
}

func TestMatrixHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Error(response, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Matrix(context.Background(), MatrixRequest{
		Sources: []Location{{Lat: 1, Lon: 2}},
		Targets: []Location{{Lat: 3, Lon: 4}},
	})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status code: %d", httpErr.StatusCode)
	}
}

func TestMatrixInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Matrix(context.Background(), MatrixRequest{
		Sources: []Location{{Lat: 1, Lon: 2}},
		Targets: []Location{{Lat: 3, Lon: 4}},
	})
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestMatrixServiceUnavailable(t *testing.T) {
	client, err := NewClient(Config{
		BaseURL: "http://localhost:8002",
		HTTPClient: &http.Client{
			Transport: failingTransport{},
		},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Matrix(context.Background(), MatrixRequest{
		Sources: []Location{{Lat: 1, Lon: 2}},
		Targets: []Location{{Lat: 3, Lon: 4}},
	})
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Fatalf("expected ErrServiceUnavailable, got %v", err)
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}
