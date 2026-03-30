# OpenSky Network API

This repository contains a community API client implementation in Golang for the [OpenSky Network](https://opensky-network.org/).
It is used to retrieve live and historical details about aircraft positioning and flight information.

The library is based on the [REST API docs](https://openskynetwork.github.io/opensky-api/rest.html).

## Installation

```
go get github.com/invander/go-opensky-api
```

The library relies on the stdlib only, so no further dependencies are required.

## Authentication

OpenSky uses **OAuth2 client credentials** for authenticated access.
Basic Auth (username/password) is no longer accepted.

1. Log in at [opensky-network.org](https://opensky-network.org) and visit the [Account](https://opensky-network.org/my-opensky/account) page.
2. Create a new API client and retrieve your `client_id` and `client_secret`.

Anonymous (unauthenticated) access is still possible but rate-limited.
Refer to the [limitations](https://openskynetwork.github.io/opensky-api/rest.html#limitations) for details.

## Usage

### Create Client

```go
// Anonymous (rate-limited)
client := opensky.NewClient()

// Authenticated (OAuth2 client credentials)
client := opensky.NewClient(
    opensky.WithCredentials("your_client_id", "your_client_secret"),
)

// With custom HTTP client
client := opensky.NewClient(
    opensky.WithCredentials("your_client_id", "your_client_secret"),
    opensky.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
)
```

### Get Flights

```go
flights, err := client.GetFlights(time.Now().Add(-2*time.Hour), time.Now())
if err != nil {
    // Check for rate limiting
    var rle *opensky.RateLimitError
    if errors.As(err, &rle) {
        fmt.Printf("rate limited, retry after %d seconds\n", rle.RetryAfterSeconds)
    }
}
fmt.Printf("received %d flight objects\n", len(flights))
```

### Get State Vectors

```go
// All current states
states, err := client.GetStates(time.Time{}, nil, nil, false)

// Filtered by ICAO24 addresses
states, err := client.GetStates(time.Time{}, []string{"3c6444", "3e1bf9"}, nil, false)

// Filtered by bounding box (Switzerland)
bbox := &opensky.BoundingBox{LaMin: 45.8389, LoMin: 5.9962, LaMax: 47.8229, LoMax: 10.5226}
states, err := client.GetStates(time.Time{}, nil, bbox, false)

// With extended aircraft category info
states, err := client.GetStates(time.Time{}, nil, nil, true)
```

### Get Track by Aircraft

```go
track, err := client.GetTrackByAircraft("3c4b26", time.Time{})
```

## Supported Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GetStates` | `GET /states/all` | All state vectors |
| `GetOwnStates` | `GET /states/own` | Own sensor state vectors (auth required) |
| `GetFlights` | `GET /flights/all` | Flights in time interval |
| `GetFlightsByAircraft` | `GET /flights/aircraft` | Flights by aircraft |
| `GetFlightsByArrival` | `GET /flights/arrival` | Arrivals by airport |
| `GetFlightsByDeparture` | `GET /flights/departure` | Departures by airport |
| `GetTrackByAircraft` | `GET /tracks` | Track by aircraft |

## Rate Limiting

The API uses a credit-based system. When credits are exhausted, methods return a `*RateLimitError`:

```go
var rle *opensky.RateLimitError
if errors.As(err, &rle) {
    fmt.Printf("retry after %d seconds, remaining: %d\n", rle.RetryAfterSeconds, rle.Remaining)
}
```