package opensky

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	baseOpenSkyURL = "https://opensky-network.org/api"
)

// Client An OpenSky API client.
type Client struct {
	username   string
	password   string
	httpClient *http.Client
}

// NewClient Creates a new OpenSky client.
// Username and password fields are optional.
func NewClient(username string, password string) *Client {
	return &Client{
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: time.Minute * 5,
		},
	}
}

// Creates a new HTTP request, with the basic authentication header already set.
func (c *Client) newRequest(method string, apiURL string) (request *http.Request, err error) {
	request, err = http.NewRequest(method, apiURL, nil)
	if err != nil {
		return
	}

	request.Header.Set("Accept", "application/json; charset=utf-8")

	if request != nil && c.username != "" && c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}
	return
}

// doHTTP is a utility method for performing an HTTP request and parsing the
// JSON response inside the passed responseObject.
//
// If the operation fails for any reason, an error is returned.
// If the HTTP request returns any status code other than 200, an error is returned.
func (c *Client) doHTTP(request *http.Request, responseObject interface{}) (err error) {
	var resp *http.Response
	resp, err = c.httpClient.Do(request)
	if err != nil {
		return
	}
	// Parse response
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errRes errorResponse
		if err = json.NewDecoder(resp.Body).Decode(&errRes); err == nil {
			return errors.New(errRes.Message)
		}

		return fmt.Errorf("unknown error, status code: %d", resp.StatusCode)
	}

	if err = json.NewDecoder(resp.Body).Decode(&responseObject); err != nil {
		return err
	}

	return nil
}

// GetFlights retrieves all flight information within a certain time interval.
// Flights departed and arrived within the [begin, end] boundaries will be returned.
//
// If no flights were found for the given time period, a 404 error will be returned instead.
func (c *Client) GetFlights(begin time.Time, end time.Time) (flights []Flight, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/flights/all", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	// Add optional parameters
	if !begin.IsZero() {
		q.Set("begin", fmt.Sprintf("%v", begin.Unix()))
	}
	if !end.IsZero() {
		q.Set("end", fmt.Sprintf("%v", end.Unix()))
	}
	request.URL.RawQuery = q.Encode()
	// Fetch response
	err = c.doHTTP(request, &flights)
	return
}

// GetFlightsByAircraft retrieves flight information for a particular aircraft, identified by the icao24 address parameter,
// within a certain time interval.
// Flights departed and arrived within the [begin, end] boundaries will be returned.
//
// If no flights were found for the given time period, a 404 error will be returned instead.
func (c *Client) GetFlightsByAircraft(icao24 string, begin time.Time, end time.Time) (flights []Flight, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/flights/aircraft", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	// Add optional parameters
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
	// Fetch response
	err = c.doHTTP(request, &flights)
	return
}

// GetFlightsByInterval retrieves flights for a certain time interval [begin, end].
//
// If no flights were found for the given time period, a 404 error will be returned instead.
func (c *Client) GetFlightsByInterval(begin time.Time, end time.Time) (flights []Flight, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/flights/all", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	// Add optional parameters
	if !begin.IsZero() {
		q.Set("begin", fmt.Sprintf("%v", begin.Unix()))
	}
	if !end.IsZero() {
		q.Set("end", fmt.Sprintf("%v", end.Unix()))
	}

	request.URL.RawQuery = q.Encode()
	// Fetch response
	err = c.doHTTP(request, &flights)
	return
}

// GetFlightsByArrival retrieve flights for a certain airport which arrived within a given time interval [begin, end].
//
// If no flights were found for the given time period, a 404 error will be returned instead.
func (c *Client) GetFlightsByArrival(airport string, begin time.Time, end time.Time) (flights []Flight, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/flights/arrival", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	// Add optional parameters
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
	// Fetch response
	err = c.doHTTP(request, &flights)
	return
}

// GetFlightsByDeparture retrieve flights for a certain airport which departed within a given time interval [begin, end].
//
// If no flights were found for the given time period, a 404 error will be returned instead.
func (c *Client) GetFlightsByDeparture(airport string, begin time.Time, end time.Time) (flights []Flight, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/flights/departure", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	// Add optional parameters
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
	// Fetch response
	err = c.doHTTP(request, &flights)
	return
}

// GetTrackByAircraft Retrieve the trajectory for a certain aircraft at a given time.
// The trajectory is a list of waypoints containing position, barometric altitude, true track and an on-ground flag.
//
// If no flights were found for the given time period, a 404 error will be returned instead.
func (c *Client) GetTrackByAircraft(icao24 string, time time.Time) (response GetTracksResponse, err error) {
	request, err := c.newRequest("GET", fmt.Sprintf("%s/tracks/all", baseOpenSkyURL))
	if err != nil {
		return
	}
	q := request.URL.Query()
	// Add optional parameters
	if icao24 != "" {
		q.Set("icao24", icao24)
	}
	if !time.IsZero() {
		q.Set("time", fmt.Sprintf("%v", time.Unix()))
	}

	request.URL.RawQuery = q.Encode()
	// Fetch response
	var rawResponse unstructuredTrackResponse
	err = c.doHTTP(request, &rawResponse)
	if err != nil {
		return
	}
	return parseTracksResponse(rawResponse)
}

// Parses an unstructured track response.
func parseTracksResponse(rawResponse unstructuredTrackResponse) (response GetTracksResponse, err error) {
	response.Icao24 = rawResponse.Icao24
	response.Callsign = rawResponse.Callsign
	response.StartTime = time.Unix(int64(rawResponse.StartTime), 0)
	response.EndTime = time.Unix(int64(rawResponse.EndTime), 0)
	// Parse state vectors
	for i, s := range rawResponse.Paths {
		var waypoint Waypoint
		waypoint, err = parseWaypoint(s, i)
		if err != nil {
			return
		}
		// Add state
		response.Paths = append(response.Paths, waypoint)
	}
	return
}

// Parse a single waypoint array from an unstructured track response.
// The i parameter represents the index of the track element in the track response.
func parseWaypoint(s []interface{}, i int) (waypoint Waypoint, err error) {
	if len(s) < 6 {
		err = fmt.Errorf("invalid waypoint object at position %v: response contains %v values, expected 17", i, len(s))
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
		err = fmt.Errorf("invalid on_ground value at position %d: %v", i, s[8])
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

// Helper function to convert a number received in a json object to an int64 type.
// Throws an error, if the number could not be parsed.
func numberToInt(val interface{}) (i int64, err error) {
	fVal, ok := val.(float64)
	if !ok {
		err = fmt.Errorf("couldn't parse %v as number", val)
		return
	}
	i = int64(fVal)
	return
}
