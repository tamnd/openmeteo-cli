// Package openmeteo exposes the Open-Meteo weather API as a kit Domain driver.
//
// A multi-domain host (ant) enables it with a single blank import:
//
//	import _ "github.com/tamnd/openmeteo-cli/openmeteo"
//
// The same Domain also builds the standalone openmeteo binary (see cli.NewApp).
package openmeteo

import (
	"context"
	"fmt"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the openmeteo driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "openmeteo",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "openmeteo",
			Short:  "Current weather and forecasts from Open-Meteo",
			Long: `openmeteo fetches current conditions and multi-day daily forecasts
from the free Open-Meteo API (api.open-meteo.com).
No API key or registration required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/openmeteo-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// current: fetch current weather conditions
	kit.Handle(app, kit.OpMeta{
		Name:    "current",
		Group:   "read",
		Single:  true,
		Summary: "Fetch current weather conditions for a location",
	}, currentOp)

	// forecast: fetch daily forecast
	kit.Handle(app, kit.OpMeta{
		Name:    "forecast",
		Group:   "read",
		List:    true,
		Summary: "Fetch daily weather forecast for a location",
	}, forecastOp)

	// geocode: resolve city name to lat/lon
	kit.Handle(app, kit.OpMeta{
		Name:    "geocode",
		Group:   "read",
		List:    true,
		Summary: "Geocode a city name to lat/lon coordinates",
	}, geocodeOp)

	// weather: current weather by city name (geocodes first)
	kit.Handle(app, kit.OpMeta{
		Name:    "weather",
		Group:   "read",
		Single:  true,
		Summary: "Fetch current weather for a city (geocodes automatically)",
	}, weatherOp)

	// hourly: hourly forecast by city name
	kit.Handle(app, kit.OpMeta{
		Name:    "hourly",
		Group:   "read",
		List:    true,
		Summary: "Fetch hourly weather forecast for a city",
	}, hourlyOp)

	// air: air quality by city name
	kit.Handle(app, kit.OpMeta{
		Name:    "air",
		Group:   "read",
		Single:  true,
		Summary: "Fetch current air quality for a city",
	}, airOp)
}

// newClient builds the client from host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type currentInput struct {
	Lat    float64       `kit:"flag" help:"latitude in decimal degrees"`
	Lon    float64       `kit:"flag" help:"longitude in decimal degrees"`
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

type forecastInput struct {
	Lat    float64       `kit:"flag" help:"latitude in decimal degrees"`
	Lon    float64       `kit:"flag" help:"longitude in decimal degrees"`
	Days   int           `kit:"flag,inherit" help:"forecast days (1-16)"`
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

type geocodeInput struct {
	City   string  `kit:"arg" help:"city name to geocode"`
	Count  int     `kit:"flag" help:"max number of results"`
	Client *Client `kit:"inject"`
}

type weatherInput struct {
	City   string  `kit:"arg" help:"city name"`
	Client *Client `kit:"inject"`
}

type hourlyInput struct {
	City   string  `kit:"arg" help:"city name"`
	Days   int     `kit:"flag" help:"forecast days (1-16, default 3)"`
	Client *Client `kit:"inject"`
}

type airInput struct {
	City   string  `kit:"arg" help:"city name"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func currentOp(ctx context.Context, in currentInput, emit func(*CurrentWeather) error) error {
	w, err := in.Client.Current(ctx, in.Lat, in.Lon)
	if err != nil {
		return mapErr(err)
	}
	return emit(w)
}

func forecastOp(ctx context.Context, in forecastInput, emit func(DailyForecast) error) error {
	days := in.Days
	if days <= 0 {
		days = 7
	}
	items, err := in.Client.Forecast(ctx, in.Lat, in.Lon, days)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func geocodeOp(ctx context.Context, in geocodeInput, emit func(GeoResult) error) error {
	count := in.Count
	if count <= 0 {
		count = 5
	}
	results, err := in.Client.Geocode(ctx, in.City, count)
	if err != nil {
		return mapErr(err)
	}
	for _, r := range results {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

func weatherOp(ctx context.Context, in weatherInput, emit func(*CurrentWeather) error) error {
	geo, err := in.Client.Geocode(ctx, in.City, 1)
	if err != nil {
		return mapErr(err)
	}
	if len(geo) == 0 {
		return fmt.Errorf("city not found: %s", in.City)
	}
	w, err := in.Client.Current(ctx, geo[0].Latitude, geo[0].Longitude)
	if err != nil {
		return mapErr(err)
	}
	return emit(w)
}

func hourlyOp(ctx context.Context, in hourlyInput, emit func(HourlySlice) error) error {
	geo, err := in.Client.Geocode(ctx, in.City, 1)
	if err != nil {
		return mapErr(err)
	}
	if len(geo) == 0 {
		return fmt.Errorf("city not found: %s", in.City)
	}
	days := in.Days
	if days <= 0 {
		days = 3
	}
	slices, err := in.Client.HourlyForecast(ctx, geo[0].Latitude, geo[0].Longitude, days)
	if err != nil {
		return mapErr(err)
	}
	for _, s := range slices {
		if err := emit(s); err != nil {
			return err
		}
	}
	return nil
}

func airOp(ctx context.Context, in airInput, emit func(AirQuality) error) error {
	geo, err := in.Client.Geocode(ctx, in.City, 1)
	if err != nil {
		return mapErr(err)
	}
	if len(geo) == 0 {
		return fmt.Errorf("city not found: %s", in.City)
	}
	aq, err := in.Client.AirQualityAt(ctx, geo[0].Latitude, geo[0].Longitude)
	if err != nil {
		return mapErr(err)
	}
	return emit(aq)
}

// --- Resolver ---

// Classify turns an input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("empty openmeteo reference")
	}
	return "weather", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "weather":
		return "https://" + Host + "/v1/forecast?" + id, nil
	default:
		return "", errs.Usage("openmeteo has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind.
func mapErr(err error) error {
	return err
}
