package opensky

import (
	"bytes"
	"strings"
	"time"
)

type errorResponse struct {
	Timestamp int64  `json:"timestamp"`
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Exception string `json:"exception"`
	Message   string `json:"message"`
	Path      string `json:"path"`
}

type CallSignTrim string

type Flight struct {
	Icao24                           string       `json:"icao24"`
	FirstSeen                        int          `json:"firstSeen"`
	EstDepartureAirport              string       `json:"estDepartureAirport"`
	LastSeen                         int          `json:"lastSeen"`
	EstArrivalAirport                string       `json:"estArrivalAirport"`
	Callsign                         CallSignTrim `json:"callsign"`
	EstDepartureAirportHorizDistance int          `json:"estDepartureAirportHorizDistance"`
	EstDepartureAirportVertDistance  int          `json:"estDepartureAirportVertDistance"`
	EstArrivalAirportHorizDistance   int          `json:"estArrivalAirportHorizDistance"`
	EstArrivalAirportVertDistance    int          `json:"estArrivalAirportVertDistance"`
	DepartureAirportCandidatesCount  int          `json:"departureAirportCandidatesCount"`
	ArrivalAirportCandidatesCount    int          `json:"arrivalAirportCandidatesCount"`
}

func (c *CallSignTrim) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, "\"")
	data = bytes.Trim(data, " ")
	*c = CallSignTrim(strings.TrimSpace(string(data)))
	return nil
}

// Unstructured raw response for tracks queries.
type unstructuredTrackResponse struct {
	Icao24    string          `json:"icao24"`
	Callsign  string          `json:"callsign"`
	StartTime float64         `json:"startTime"`
	EndTime   float64         `json:"endTime"`
	Paths     [][]interface{} `json:"path"`
}

type GetTracksResponse struct {
	Icao24    string     `json:"icao24"`
	Callsign  string     `json:"callsign"`
	StartTime time.Time  `json:"startTime"`
	EndTime   time.Time  `json:"endTime"`
	Paths     []Waypoint `json:"path"`
}

type Waypoint struct {
	Time         time.Time `json:"time"`
	Latitude     *float64  `json:"latitude"`
	Longitude    *float64  `json:"longitude"`
	BaroAltitude *float64  `json:"baro_altitude"`
	TrueTrack    *float64  `json:"true_track"`
	OnGround     bool      `json:"on_ground"`
}
