package opensky

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient creates a Client that points at the given test server.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return &Client{httpClient: srv.Client()}
}

// newTestClientWithURL creates a Client pointed at srv and patches baseURL for requests.
// Since baseOpenSkyURL is a const, we route through a mux that maps API paths.
func setupMux(handlers map[string]http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	for path, handler := range handlers {
		mux.HandleFunc(path, handler)
	}
	return httptest.NewServer(mux)
}

// --- doHTTP tests ---

func TestDoHTTP_RateLimitError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Rate-Limit-Retry-After-Seconds", "42")
		w.Header().Set("X-Rate-Limit-Remaining", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	req, _ := http.NewRequest("GET", srv.URL, nil)

	var result any
	err := client.doHTTP(req, &result)

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
	if rle.RetryAfterSeconds != 42 {
		t.Errorf("expected RetryAfterSeconds=42, got %d", rle.RetryAfterSeconds)
	}
	if rle.Remaining != 0 {
		t.Errorf("expected Remaining=0, got %d", rle.Remaining)
	}
}

func TestDoHTTP_404ReturnsNil(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	req, _ := http.NewRequest("GET", srv.URL, nil)

	var flights []Flight
	err := client.doHTTP(req, &flights)
	if err != nil {
		t.Fatalf("expected nil error for 404, got: %v", err)
	}
	if flights != nil {
		t.Errorf("expected nil result for 404, got: %v", flights)
	}
}

func TestDoHTTP_ServerErrorWithMessage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(errorResponse{Message: "access denied"})
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	req, _ := http.NewRequest("GET", srv.URL, nil)

	var result any
	err := client.doHTTP(req, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "HTTP 403: access denied"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestDoHTTP_ServerErrorWithoutMessage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	req, _ := http.NewRequest("GET", srv.URL, nil)

	var result any
	err := client.doHTTP(req, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "HTTP 502: unexpected error"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

// --- GetFlights ---

func TestGetFlights(t *testing.T) {
	t.Parallel()

	flightsJSON := `[
		{"icao24":"abc123","firstSeen":1000,"estDepartureAirport":"EDDF","lastSeen":2000,"estArrivalAirport":"EGLL","callsign":"DLH123"},
		{"icao24":"def456","firstSeen":1100,"estDepartureAirport":"LFPG","lastSeen":2100,"estArrivalAirport":"LEMD","callsign":"AFR456"}
	]`

	srv := setupMux(map[string]http.HandlerFunc{
		"/flights/all": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("begin") == "" || r.URL.Query().Get("end") == "" {
				t.Error("expected begin and end query params")
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(flightsJSON))
		},
	})
	defer srv.Close()

	client := newTestClient(t, srv)

	req, _ := client.newRequest("GET", srv.URL+"/flights/all")
	q := req.URL.Query()
	q.Set("begin", "1000")
	q.Set("end", "2000")
	req.URL.RawQuery = q.Encode()

	var flights []Flight
	err := client.doHTTP(req, &flights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flights) != 2 {
		t.Fatalf("expected 2 flights, got %d", len(flights))
	}
	if flights[0].Icao24 != "abc123" {
		t.Errorf("expected icao24 'abc123', got %q", flights[0].Icao24)
	}
	if string(flights[1].Callsign) != "AFR456" {
		t.Errorf("expected callsign 'AFR456', got %q", flights[1].Callsign)
	}
}

// --- GetStates ---

func TestGetStates_ParsesStateVectors(t *testing.T) {
	t.Parallel()

	statesJSON := `{
		"time": 1700000000,
		"states": [
			["abc123","DLH123 ","Germany",1700000000,1700000000,8.5,50.0,10000.0,false,250.0,180.0,-5.0,null,10500.0,"1234",false,0,3]
		]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(statesJSON))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	req, _ := http.NewRequest("GET", srv.URL, nil)

	var raw unstructuredStatesResponse
	err := client.doHTTP(req, &raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	response, err := parseStatesResponse(raw)
	if err != nil {
		t.Fatalf("parseStatesResponse error: %v", err)
	}
	if response.Time != 1700000000 {
		t.Errorf("expected time 1700000000, got %d", response.Time)
	}
	if len(response.States) != 1 {
		t.Fatalf("expected 1 state vector, got %d", len(response.States))
	}

	sv := response.States[0]
	if sv.Icao24 != "abc123" {
		t.Errorf("expected icao24 'abc123', got %q", sv.Icao24)
	}
	if sv.Callsign == nil || *sv.Callsign != "DLH123" {
		t.Errorf("expected callsign 'DLH123', got %v", sv.Callsign)
	}
	if sv.OriginCountry != "Germany" {
		t.Errorf("expected country 'Germany', got %q", sv.OriginCountry)
	}
	if sv.OnGround != false {
		t.Error("expected on_ground=false")
	}
	if sv.Longitude == nil || *sv.Longitude != 8.5 {
		t.Errorf("expected longitude 8.5, got %v", sv.Longitude)
	}
	if sv.Latitude == nil || *sv.Latitude != 50.0 {
		t.Errorf("expected latitude 50.0, got %v", sv.Latitude)
	}
	if sv.PositionSource != PositionSourceADSB {
		t.Errorf("expected position_source 0 (ADS-B), got %d", sv.PositionSource)
	}
	if sv.Category != CategorySmall {
		t.Errorf("expected category 3 (Small), got %d", sv.Category)
	}
	if sv.Squawk == nil || *sv.Squawk != "1234" {
		t.Errorf("expected squawk '1234', got %v", sv.Squawk)
	}
}

func TestParseStateVector_TooFewFields(t *testing.T) {
	t.Parallel()

	short := []any{"abc123", "DLH123", "Germany"}
	_, err := parseStateVector(short, 0)
	if err == nil {
		t.Fatal("expected error for short state vector, got nil")
	}
}

func TestParseStateVector_NullableFields(t *testing.T) {
	t.Parallel()

	// State vector with many null fields.
	sv := []any{
		"abc123", nil, "Germany",
		nil, float64(1700000000),
		nil, nil, nil,
		true,
		nil, nil, nil,
		nil,
		nil, nil,
		false, float64(0),
	}

	result, err := parseStateVector(sv, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Callsign != nil {
		t.Errorf("expected nil callsign, got %v", result.Callsign)
	}
	if result.TimePosition != nil {
		t.Errorf("expected nil time_position, got %v", result.TimePosition)
	}
	if result.Longitude != nil {
		t.Errorf("expected nil longitude, got %v", result.Longitude)
	}
	if !result.OnGround {
		t.Error("expected on_ground=true")
	}
}

// --- Track parsing ---

func TestParseTracksResponse(t *testing.T) {
	t.Parallel()

	raw := unstructuredTrackResponse{
		Icao24:    "abc123",
		Callsign:  "DLH123",
		StartTime: 1700000000,
		EndTime:   1700003600,
		Paths: [][]any{
			{float64(1700000000), float64(50.0), float64(8.5), float64(10000.0), float64(180.0), true},
			{float64(1700001000), float64(51.0), float64(9.0), nil, float64(90.0), false},
		},
	}

	response, err := parseTracksResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.Icao24 != "abc123" {
		t.Errorf("expected icao24 'abc123', got %q", response.Icao24)
	}
	if response.Callsign != "DLH123" {
		t.Errorf("expected callsign 'DLH123', got %q", response.Callsign)
	}
	if len(response.Paths) != 2 {
		t.Fatalf("expected 2 waypoints, got %d", len(response.Paths))
	}

	wp := response.Paths[0]
	if wp.Latitude == nil || *wp.Latitude != 50.0 {
		t.Errorf("expected latitude 50.0, got %v", wp.Latitude)
	}
	if !wp.OnGround {
		t.Error("expected on_ground=true for first waypoint")
	}

	wp2 := response.Paths[1]
	if wp2.BaroAltitude != nil {
		t.Errorf("expected nil baro_altitude for second waypoint, got %v", wp2.BaroAltitude)
	}
	if wp2.OnGround {
		t.Error("expected on_ground=false for second waypoint")
	}
}

func TestParseWaypoint_TooFewFields(t *testing.T) {
	t.Parallel()

	short := []any{float64(1700000000), float64(50.0)}
	_, err := parseWaypoint(short, 0)
	if err == nil {
		t.Fatal("expected error for short waypoint, got nil")
	}
}

func TestParseWaypoint_InvalidOnGround(t *testing.T) {
	t.Parallel()

	bad := []any{float64(1700000000), float64(50.0), float64(8.5), float64(10000.0), float64(180.0), "not-a-bool"}
	_, err := parseWaypoint(bad, 0)
	if err == nil {
		t.Fatal("expected error for invalid on_ground, got nil")
	}
}

// --- CallSignTrim ---

func TestCallSignTrim_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"trimmed", `"DLH123  "`, "DLH123"},
		{"no_spaces", `"AFR456"`, "AFR456"},
		{"only_spaces", `"       "`, ""},
		{"empty_string", `""`, ""},
		{"null", `null`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var cs CallSignTrim
			if err := cs.UnmarshalJSON([]byte(tt.input)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(cs) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(cs))
			}
		})
	}
}

// --- numberToInt ---

func TestNumberToInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   any
		want    int64
		wantErr bool
	}{
		{"valid_float64", float64(42), 42, false},
		{"zero", float64(0), 0, false},
		{"negative", float64(-10), -10, false},
		{"large", float64(1700000000), 1700000000, false},
		{"string_invalid", "not-a-number", 0, true},
		{"nil_invalid", nil, 0, true},
		{"bool_invalid", true, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := numberToInt(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

// --- Bearer token on requests ---

func TestNewRequest_SetsBearer(t *testing.T) {
	t.Parallel()

	tokenSrv := newTestTokenServer(t, "my-bearer-token", 1800)
	defer tokenSrv.Close()

	client := &Client{
		httpClient: tokenSrv.Client(),
		tokenManager: &tokenManager{
			clientID:     "id",
			clientSecret: "secret",
			httpClient:   tokenSrv.Client(),
			tokenURL:     tokenSrv.URL,
		},
	}

	req, err := client.newRequest("GET", "https://example.com/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	auth := req.Header.Get("Authorization")
	if auth != "Bearer my-bearer-token" {
		t.Errorf("expected 'Bearer my-bearer-token', got %q", auth)
	}
}

func TestNewRequest_NoAuthWhenAnonymous(t *testing.T) {
	t.Parallel()

	client := NewClient()
	req, err := client.newRequest("GET", "https://example.com/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	auth := req.Header.Get("Authorization")
	if auth != "" {
		t.Errorf("expected no Authorization header for anonymous client, got %q", auth)
	}
}

// --- RateLimitError ---

func TestRateLimitError_ErrorString(t *testing.T) {
	t.Parallel()

	rle := &RateLimitError{RetryAfterSeconds: 60, Remaining: 0}
	expected := "rate limit exceeded, retry after 60 seconds"
	if rle.Error() != expected {
		t.Errorf("expected %q, got %q", expected, rle.Error())
	}
}

// --- NewClient options ---

func TestNewClient_Anonymous(t *testing.T) {
	t.Parallel()

	client := NewClient()
	if client.httpClient == nil {
		t.Fatal("expected non-nil httpClient")
	}
	if client.tokenManager != nil {
		t.Error("expected nil tokenManager for anonymous client")
	}
}

func TestNewClient_WithCredentials(t *testing.T) {
	t.Parallel()

	client := NewClient(WithCredentials("id", "secret"))
	if client.tokenManager == nil {
		t.Fatal("expected non-nil tokenManager")
	}
	if client.tokenManager.clientID != "id" {
		t.Errorf("expected clientID 'id', got %q", client.tokenManager.clientID)
	}
	if client.tokenManager.clientSecret != "secret" {
		t.Errorf("expected clientSecret 'secret', got %q", client.tokenManager.clientSecret)
	}
}

func TestNewClient_OptionOrdering(t *testing.T) {
	t.Parallel()

	customHTTP := &http.Client{Timeout: 10 * time.Second}

	// WithHTTPClient AFTER WithCredentials — should propagate to tokenManager.
	client := NewClient(
		WithCredentials("id", "secret"),
		WithHTTPClient(customHTTP),
	)
	if client.httpClient != customHTTP {
		t.Error("expected custom httpClient on client")
	}
	if client.tokenManager.httpClient != customHTTP {
		t.Error("expected custom httpClient propagated to tokenManager")
	}

	// WithHTTPClient BEFORE WithCredentials — tokenManager gets the custom client via c.httpClient.
	client2 := NewClient(
		WithHTTPClient(customHTTP),
		WithCredentials("id", "secret"),
	)
	if client2.tokenManager.httpClient != customHTTP {
		t.Error("expected custom httpClient on tokenManager when WithHTTPClient is first")
	}
}

// --- doHTTP edge cases ---

func TestDoHTTP_InvalidJSONOn200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	req, _ := http.NewRequest("GET", srv.URL, nil)

	var flights []Flight
	err := client.doHTTP(req, &flights)
	if err == nil {
		t.Fatal("expected error for invalid JSON on 200, got nil")
	}
}

func TestDoHTTP_EmptyResponseOn200(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	req, _ := http.NewRequest("GET", srv.URL, nil)

	var flights []Flight
	err := client.doHTTP(req, &flights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flights) != 0 {
		t.Errorf("expected empty slice, got %d items", len(flights))
	}
}
