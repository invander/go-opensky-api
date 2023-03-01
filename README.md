# OpenSky Network API

This repository contains a community API client implementation in Golang for the [OpenSky Network](https://opensky-network.org/).
It is used to retrieve live and historical details about aircraft positioning and flight information.

The library is based on the [REST API docs](https://opensky-network.org/apidoc/rest.html).

Implemented only getting historical data without State Vectors. 

## Installation

```
go get github.com/invander/go-opensky-api
```

The library relies on the stdlib only, so no further dependencies are required.

## User Account

The client does not strictly require an account to use the OpenSky API. Username and password are, therefore, optional!

Refer to the [limitations](https://opensky-network.org/apidoc/rest.html#limitations), to see why/when a user account would be preferred.

## Usage

Create your API client:
```go
client := opensky.NewClient("myusername", "mypassword")
```

### Get Flights

```go
flights, err := client.GetFlights(time.Now().Add(-2*time.Hour), time.Now())
if err != nil {
    // Something went wrong, check the error
}
fmt.Printf("received %d flight objects", len(flights))
for _, flight := range flights {
	// Check the contents of each received flight
}
```