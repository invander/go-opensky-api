package opensky

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	baseOpenSkyURL = "https://opensky-network.org/api"

	headerRateLimitRemaining  = "X-Rate-Limit-Remaining"
	headerRateLimitRetryAfter = "X-Rate-Limit-Retry-After-Seconds"
)

// RateLimitError is returned when the API responds with 429 Too Many Requests.
type RateLimitError struct {
	// RetryAfterSeconds indicates how many seconds to wait before retrying.
	RetryAfterSeconds int
	// Remaining credits at the time of the response (may be 0).
	Remaining int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded, retry after %d seconds", e.RetryAfterSeconds)
}

// Option configures the Client.
type Option func(*Client)

// WithCredentials sets OAuth2 client credentials for authenticated API access.
func WithCredentials(clientID, clientSecret string) Option {
	return func(c *Client) {
		c.tokenManager = newTokenManager(clientID, clientSecret, c.httpClient)
	}
}

// WithHTTPClient sets a custom http.Client for all requests (including token refresh).
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
		if c.tokenManager != nil {
			c.tokenManager.httpClient = httpClient
		}
	}
}

// Client is an OpenSky API client.
// Create one with NewClient and functional options.
type Client struct {
	httpClient   *http.Client
	tokenManager *tokenManager
}

// NewClient creates a new OpenSky client.
// Without options the client works anonymously (rate-limited).
//
//	Anonymous:     opensky.NewClient()
//	Authenticated: opensky.NewClient(opensky.WithCredentials("client_id", "client_secret"))
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// newRequest creates a new HTTP request with the appropriate authorization header.
func (c *Client) newRequest(method string, apiURL string) (request *http.Request, err error) {
	request, err = http.NewRequest(method, apiURL, nil)
	if err != nil {
		return
	}

	request.Header.Set("Accept", "application/json; charset=utf-8")

	if c.tokenManager != nil {
		var token string
		token, err = c.tokenManager.getToken()
		if err != nil {
			return nil, fmt.Errorf("obtaining access token: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return
}

// doHTTP performs an HTTP request and decodes the JSON response into responseObject.
//
// Returns a *RateLimitError when the API responds with 429.
// Returns an error for any non-200 status code.
func (c *Client) doHTTP(request *http.Request, responseObject any) (err error) {
	var resp *http.Response
	resp, err = c.httpClient.Do(request)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		rle := &RateLimitError{}
		if v := resp.Header.Get(headerRateLimitRetryAfter); v != "" {
			rle.RetryAfterSeconds, _ = strconv.Atoi(v)
		}
		if v := resp.Header.Get(headerRateLimitRemaining); v != "" {
			rle.Remaining, _ = strconv.Atoi(v)
		}
		return rle
	}

	// 404 means "no data found" for flights and tracks endpoints.
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		var errRes errorResponse
		if err = json.NewDecoder(resp.Body).Decode(&errRes); err == nil && errRes.Message != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errRes.Message)
		}

		return fmt.Errorf("HTTP %d: unexpected error", resp.StatusCode)
	}

	if err = json.NewDecoder(resp.Body).Decode(&responseObject); err != nil {
		return err
	}

	return nil
}

// GetFlights retrieves all flight information within a certain time interval.
// Flights departed and arrived within the [begin, end] boundaries will be returned.
//
// Returns an empty slice if no flights were found for the given time period.
func (c *Client) GetFlights(begin time.Time, end time.Time) (flights []Flight, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/flights/all", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	if !begin.IsZero() {
		q.Set("begin", fmt.Sprintf("%v", begin.Unix()))
	}
	if !end.IsZero() {
		q.Set("end", fmt.Sprintf("%v", end.Unix()))
	}

	request.URL.RawQuery = q.Encode()
	err = c.doHTTP(request, &flights)
	return
}

// GetFlightsByAircraft retrieves flight information for a particular aircraft, identified by the icao24 address parameter,
// within a certain time interval.
// Flights departed and arrived within the [begin, end] boundaries will be returned.
//
// Returns an empty slice if no flights were found for the given time period.
func (c *Client) GetFlightsByAircraft(icao24 string, begin time.Time, end time.Time) (flights []Flight, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/flights/aircraft", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	if !begin.IsZero() {
		q.Set("begin", fmt.Sprintf("%v", begin.Unix()))
	}
	if !end.IsZero() {
		q.Set("end", fmt.Sprintf("%v", end.Unix()))
	}
	if icao24 != "" {
		q.Set("icao24", icao24)
	}

	request.URL.RawQuery = q.Encode()
	err = c.doHTTP(request, &flights)
	return
}

// GetFlightsByInterval retrieves flights for a certain time interval [begin, end].
// Deprecated: Use GetFlights instead, which provides the same functionality.
//
// Returns an empty slice if no flights were found for the given time period.
func (c *Client) GetFlightsByInterval(begin time.Time, end time.Time) (flights []Flight, err error) {
	return c.GetFlights(begin, end)
}

// GetFlightsByArrival retrieves flights for a certain airport which arrived within a given time interval [begin, end].
//
// Returns an empty slice if no flights were found for the given time period.
func (c *Client) GetFlightsByArrival(airport string, begin time.Time, end time.Time) (flights []Flight, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/flights/arrival", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	if !begin.IsZero() {
		q.Set("begin", fmt.Sprintf("%v", begin.Unix()))
	}
	if !end.IsZero() {
		q.Set("end", fmt.Sprintf("%v", end.Unix()))
	}
	if airport != "" {
		q.Set("airport", airport)
	}

	request.URL.RawQuery = q.Encode()
	err = c.doHTTP(request, &flights)
	return
}

// GetFlightsByDeparture retrieves flights for a certain airport which departed within a given time interval [begin, end].
//
// Returns an empty slice if no flights were found for the given time period.
func (c *Client) GetFlightsByDeparture(airport string, begin time.Time, end time.Time) (flights []Flight, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/flights/departure", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	if !begin.IsZero() {
		q.Set("begin", fmt.Sprintf("%v", begin.Unix()))
	}
	if !end.IsZero() {
		q.Set("end", fmt.Sprintf("%v", end.Unix()))
	}
	if airport != "" {
		q.Set("airport", airport)
	}

	request.URL.RawQuery = q.Encode()
	err = c.doHTTP(request, &flights)
	return
}

// GetTrackByAircraft retrieves the trajectory for a certain aircraft at a given time.
// The trajectory is a list of waypoints containing position, barometric altitude, true track and an on-ground flag.
//
// Returns an empty response if no track data was found.
func (c *Client) GetTrackByAircraft(icao24 string, t time.Time) (response GetTracksResponse, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/tracks", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	if icao24 != "" {
		q.Set("icao24", icao24)
	}
	if !t.IsZero() {
		q.Set("time", fmt.Sprintf("%v", t.Unix()))
	}

	request.URL.RawQuery = q.Encode()

	var rawResponse unstructuredTrackResponse
	err = c.doHTTP(request, &rawResponse)
	if err != nil {
		return
	}
	return parseTracksResponse(rawResponse)
}

// parseTracksResponse converts a raw unstructured track response into a typed GetTracksResponse.
func parseTracksResponse(rawResponse unstructuredTrackResponse) (response GetTracksResponse, err error) {
	response.Icao24 = rawResponse.Icao24
	response.Callsign = rawResponse.Callsign
	response.StartTime = time.Unix(int64(rawResponse.StartTime), 0)
	response.EndTime = time.Unix(int64(rawResponse.EndTime), 0)
	for i, s := range rawResponse.Paths {
		var waypoint Waypoint
		waypoint, err = parseWaypoint(s, i)
		if err != nil {
			return
		}
		response.Paths = append(response.Paths, waypoint)
	}
	return
}

// parseWaypoint converts a single waypoint array from an unstructured track response into a typed Waypoint.
func parseWaypoint(s []any, i int) (waypoint Waypoint, err error) {
	if len(s) < 6 {
		err = fmt.Errorf("invalid waypoint at position %d: contains %d values, expected at least 6", i, len(s))
		return
	}

	// time
	var rawTime int64
	var parsedTime time.Time
	if s[0] != nil {
		rawTime, err = numberToInt(s[0])
		if err != nil {
			err = fmt.Errorf("invalid time_position value at position %d: %w", i, err)
			return
		}
		parsedTime = time.Unix(rawTime, 0)
	}
	// latitude
	var lat *float64
	if rawLat, ok := s[1].(float64); ok {
		lat = &rawLat
	}
	// longitude
	var lon *float64
	if rawLon, ok := s[2].(float64); ok {
		lon = &rawLon
	}
	// baro_altitude
	var baroAltitude *float64
	if rawBaroAltitude, ok := s[3].(float64); ok {
		baroAltitude = &rawBaroAltitude
	}

	// true_track
	var trueTrack *float64
	if rawTrueTrack, ok := s[4].(float64); ok {
		trueTrack = &rawTrueTrack
	}
	// on_ground
	onGround, ok := s[5].(bool)
	if !ok {
		err = fmt.Errorf("invalid on_ground value at position %d: %v", i, s[5])
		return
	}

	waypoint = Waypoint{
		Time:         parsedTime,
		Latitude:     lat,
		Longitude:    lon,
		BaroAltitude: baroAltitude,
		TrueTrack:    trueTrack,
		OnGround:     onGround,
	}
	return
}

// GetStates retrieves state vectors for all aircraft or a filtered subset.
// Parameters:
//   - t: time to retrieve states for (zero value means current time)
//   - icao24: list of ICAO24 addresses to filter (nil or empty means all aircraft)
//   - bbox: optional bounding box to filter by geographic area (nil means no area filter)
//   - extended: if true, request aircraft category information
func (c *Client) GetStates(t time.Time, icao24 []string, bbox *BoundingBox, extended bool) (response StatesResponse, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/states/all", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	if !t.IsZero() {
		q.Set("time", fmt.Sprintf("%v", t.Unix()))
	}
	for _, addr := range icao24 {
		q.Add("icao24", addr)
	}
	if bbox != nil {
		q.Set("lamin", fmt.Sprintf("%v", bbox.LaMin))
		q.Set("lomin", fmt.Sprintf("%v", bbox.LoMin))
		q.Set("lamax", fmt.Sprintf("%v", bbox.LaMax))
		q.Set("lomax", fmt.Sprintf("%v", bbox.LoMax))
	}
	if extended {
		q.Set("extended", "1")
	}
	request.URL.RawQuery = q.Encode()

	var rawResponse unstructuredStatesResponse
	err = c.doHTTP(request, &rawResponse)
	if err != nil {
		return
	}
	return parseStatesResponse(rawResponse)
}

// GetOwnStates retrieves state vectors for your own sensors.
// Requires authentication; returns 403 Forbidden without credentials.
// Parameters:
//   - t: time to retrieve states for (zero value means current time)
//   - icao24: list of ICAO24 addresses to filter (nil or empty means all aircraft)
//   - serials: list of receiver serial numbers to filter (nil or empty means all receivers)
func (c *Client) GetOwnStates(t time.Time, icao24 []string, serials []int) (response StatesResponse, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/states/own", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	if !t.IsZero() {
		q.Set("time", fmt.Sprintf("%v", t.Unix()))
	}
	for _, addr := range icao24 {
		q.Add("icao24", addr)
	}
	for _, s := range serials {
		q.Add("serials", fmt.Sprintf("%d", s))
	}
	request.URL.RawQuery = q.Encode()

	var rawResponse unstructuredStatesResponse
	err = c.doHTTP(request, &rawResponse)
	if err != nil {
		return
	}
	return parseStatesResponse(rawResponse)
}

// parseStatesResponse converts the raw two-dimensional array response into typed StatesResponse.
func parseStatesResponse(raw unstructuredStatesResponse) (response StatesResponse, err error) {
	response.Time = raw.Time
	for i, s := range raw.States {
		var sv StateVector
		sv, err = parseStateVector(s, i)
		if err != nil {
			return
		}
		response.States = append(response.States, sv)
	}
	return
}

// parseStateVector converts a single state vector array (up to 18 elements) into a typed StateVector.
func parseStateVector(s []any, i int) (sv StateVector, err error) {
	if len(s) < 17 {
		err = fmt.Errorf("invalid state vector at position %d: contains %d values, expected at least 17", i, len(s))
		return
	}

	// 0: icao24
	if v, ok := s[0].(string); ok {
		sv.Icao24 = v
	}
	// 1: callsign
	if v, ok := s[1].(string); ok {
		trimmed := strings.TrimSpace(v)
		sv.Callsign = &trimmed
	}
	// 2: origin_country
	if v, ok := s[2].(string); ok {
		sv.OriginCountry = v
	}
	// 3: time_position
	if s[3] != nil {
		raw, e := numberToInt(s[3])
		if e != nil {
			err = fmt.Errorf("invalid time_position at position %d: %w", i, e)
			return
		}
		sv.TimePosition = &raw
	}
	// 4: last_contact
	if s[4] != nil {
		sv.LastContact, err = numberToInt(s[4])
		if err != nil {
			err = fmt.Errorf("invalid last_contact at position %d: %w", i, err)
			return
		}
	}
	// 5: longitude
	if v, ok := s[5].(float64); ok {
		sv.Longitude = &v
	}
	// 6: latitude
	if v, ok := s[6].(float64); ok {
		sv.Latitude = &v
	}
	// 7: baro_altitude
	if v, ok := s[7].(float64); ok {
		sv.BaroAltitude = &v
	}
	// 8: on_ground
	if v, ok := s[8].(bool); ok {
		sv.OnGround = v
	}
	// 9: velocity
	if v, ok := s[9].(float64); ok {
		sv.Velocity = &v
	}
	// 10: true_track
	if v, ok := s[10].(float64); ok {
		sv.TrueTrack = &v
	}
	// 11: vertical_rate
	if v, ok := s[11].(float64); ok {
		sv.VerticalRate = &v
	}
	// 12: sensors
	if s[12] != nil {
		if arr, ok := s[12].([]any); ok {
			for _, elem := range arr {
				raw, e := numberToInt(elem)
				if e == nil {
					sv.Sensors = append(sv.Sensors, int(raw))
				}
			}
		}
	}
	// 13: geo_altitude
	if v, ok := s[13].(float64); ok {
		sv.GeoAltitude = &v
	}
	// 14: squawk
	if v, ok := s[14].(string); ok {
		sv.Squawk = &v
	}
	// 15: spi
	if v, ok := s[15].(bool); ok {
		sv.Spi = v
	}
	// 16: position_source
	if v, ok := s[16].(float64); ok {
		sv.PositionSource = PositionSource(int(v))
	}
	// 17: category (optional, only with extended=1)
	if len(s) > 17 {
		if v, ok := s[17].(float64); ok {
			sv.Category = AircraftCategory(int(v))
		}
	}

	return
}

// numberToInt converts a JSON number (float64) to int64.
func numberToInt(val any) (i int64, err error) {
	fVal, ok := val.(float64)
	if !ok {
		err = fmt.Errorf("couldn't parse %v as number", val)
		return
	}
	i = int64(fVal)
	return
}
