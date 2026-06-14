package openmeteo_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/openmeteo-cli/openmeteo"
)

const geoJSON = `{
  "results": [
    {
      "id": 2988507,
      "name": "Paris",
      "latitude": 48.85341,
      "longitude": 2.3488,
      "country": "France",
      "country_code": "FR",
      "timezone": "Europe/Paris"
    }
  ]
}`

const geoEmptyJSON = `{}`

const hourlyJSON = `{
  "timezone": "Europe/Paris",
  "hourly": {
    "time": ["2026-06-14T00:00", "2026-06-14T01:00", "2026-06-14T02:00"],
    "temperature_2m": [17.5, 17.0, 16.8],
    "relative_humidity_2m": [68, 70, 72],
    "precipitation_probability": [0, 5, 10],
    "wind_speed_10m": [8.3, 7.1, 6.9]
  }
}`

const airQualityJSON = `{
  "current": {
    "pm10": 9.6,
    "pm2_5": 5.9,
    "carbon_monoxide": 230.4,
    "nitrogen_dioxide": 12.3,
    "ozone": 67.8,
    "european_aqi": 22.0
  }
}`

const currentJSON = `{
  "latitude": 48.8566,
  "longitude": 2.3522,
  "timezone": "Europe/Paris",
  "current": {
    "time": "2026-06-14T11:45",
    "interval": 900,
    "temperature_2m": 22.9,
    "relative_humidity_2m": 55,
    "wind_speed_10m": 7.6,
    "wind_direction_10m": 280,
    "weathercode": 0,
    "surface_pressure": 1013.2
  }
}`

const forecastJSON = `{
  "latitude": 48.8566,
  "longitude": 2.3522,
  "timezone": "Europe/Paris",
  "daily": {
    "time": ["2026-06-14", "2026-06-15"],
    "temperature_2m_max": [26.1, 28.9],
    "temperature_2m_min": [16.2, 17.8],
    "precipitation_sum": [0.0, 0.2],
    "wind_speed_10m_max": [15.3, 12.1],
    "weathercode": [0, 1]
  }
}`

func newTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("current") != "" {
			_, _ = fmt.Fprint(w, currentJSON)
		} else {
			_, _ = fmt.Fprint(w, forecastJSON)
		}
	}))
}

func newTestClient(ts *httptest.Server) *openmeteo.Client {
	cfg := openmeteo.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return openmeteo.NewClient(cfg)
}

func TestCurrentSendsUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, currentJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Current(context.Background(), 48.8566, 2.3522)
	if err != nil {
		t.Fatal(err)
	}
	if gotUA == "" {
		t.Error("User-Agent not sent")
	}
}

func TestCurrentParsesResponse(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	c := newTestClient(ts)
	w, err := c.Current(context.Background(), 48.8566, 2.3522)
	if err != nil {
		t.Fatal(err)
	}
	if w.TempC != 22.9 {
		t.Errorf("TempC = %v, want 22.9", w.TempC)
	}
	if w.Humidity != 55 {
		t.Errorf("Humidity = %d, want 55", w.Humidity)
	}
	if w.WindKph != 7.6 {
		t.Errorf("WindKph = %v, want 7.6", w.WindKph)
	}
	if w.WindDir != 280 {
		t.Errorf("WindDir = %d, want 280", w.WindDir)
	}
	if w.Pressure != 1013.2 {
		t.Errorf("Pressure = %v, want 1013.2", w.Pressure)
	}
	if w.WeatherCode != 0 {
		t.Errorf("WeatherCode = %d, want 0", w.WeatherCode)
	}
	if w.Description != "Clear sky" {
		t.Errorf("Description = %q, want Clear sky", w.Description)
	}
	if w.Timezone != "Europe/Paris" {
		t.Errorf("Timezone = %q, want Europe/Paris", w.Timezone)
	}
}

func TestCurrentRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, currentJSON)
	}))
	defer ts.Close()

	cfg := openmeteo.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := openmeteo.NewClient(cfg)

	_, err := c.Current(context.Background(), 48.8566, 2.3522)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestForecastParsesResponse(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	c := newTestClient(ts)
	days, err := c.Forecast(context.Background(), 48.8566, 2.3522, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 2 {
		t.Fatalf("len(days) = %d, want 2", len(days))
	}
	if days[0].Date != "2026-06-14" {
		t.Errorf("days[0].Date = %q, want 2026-06-14", days[0].Date)
	}
	if days[0].TempMaxC != 26.1 {
		t.Errorf("days[0].TempMaxC = %v, want 26.1", days[0].TempMaxC)
	}
	if days[1].TempMinC != 17.8 {
		t.Errorf("days[1].TempMinC = %v, want 17.8", days[1].TempMinC)
	}
	if days[0].Description != "Clear sky" {
		t.Errorf("days[0].Description = %q, want Clear sky", days[0].Description)
	}
	if days[1].Description != "Partly cloudy" {
		t.Errorf("days[1].Description = %q, want Partly cloudy", days[1].Description)
	}
}

func TestForecastLimitRespected(t *testing.T) {
	var gotDays string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDays = r.URL.Query().Get("forecast_days")
		_, _ = fmt.Fprint(w, forecastJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Forecast(context.Background(), 48.8566, 2.3522, 2)
	if err != nil {
		t.Fatal(err)
	}
	if gotDays != "2" {
		t.Errorf("forecast_days = %q, want 2", gotDays)
	}
}

func newGeoTestClient(geoHandler, forecastHandler, airHandler http.Handler) (*openmeteo.Client, func()) {
	geoSrv := httptest.NewServer(geoHandler)
	fcSrv := httptest.NewServer(forecastHandler)
	airSrv := httptest.NewServer(airHandler)
	cfg := openmeteo.DefaultConfig()
	cfg.GeoURL = geoSrv.URL
	cfg.BaseURL = fcSrv.URL
	cfg.AirURL = airSrv.URL
	cfg.Rate = 0
	return openmeteo.NewClient(cfg), func() { geoSrv.Close(); fcSrv.Close(); airSrv.Close() }
}

func TestGeocodeParsesResponse(t *testing.T) {
	geoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") != "Paris" {
			t.Errorf("name param = %q, want Paris", r.URL.Query().Get("name"))
		}
		_, _ = fmt.Fprint(w, geoJSON)
	})
	c, cleanup := newGeoTestClient(geoHandler, http.NotFoundHandler(), http.NotFoundHandler())
	defer cleanup()

	results, err := c.Geocode(context.Background(), "Paris", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Name != "Paris" {
		t.Errorf("Name = %q, want Paris", results[0].Name)
	}
	if results[0].Latitude != 48.85341 {
		t.Errorf("Latitude = %v, want 48.85341", results[0].Latitude)
	}
	if results[0].Longitude != 2.3488 {
		t.Errorf("Longitude = %v, want 2.3488", results[0].Longitude)
	}
	if results[0].CountryCode != "FR" {
		t.Errorf("CountryCode = %q, want FR", results[0].CountryCode)
	}
}

func TestGeocodeEmpty(t *testing.T) {
	geoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, geoEmptyJSON)
	})
	c, cleanup := newGeoTestClient(geoHandler, http.NotFoundHandler(), http.NotFoundHandler())
	defer cleanup()

	results, err := c.Geocode(context.Background(), "Xyzzy", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestHourlyForecastParsesResponse(t *testing.T) {
	fcHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, hourlyJSON)
	})
	c, cleanup := newGeoTestClient(http.NotFoundHandler(), fcHandler, http.NotFoundHandler())
	defer cleanup()

	slices, err := c.HourlyForecast(context.Background(), 48.85341, 2.3488, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(slices) != 3 {
		t.Fatalf("len(slices) = %d, want 3", len(slices))
	}
	if slices[0].Time != "2026-06-14T00:00" {
		t.Errorf("slices[0].Time = %q, want 2026-06-14T00:00", slices[0].Time)
	}
	if slices[0].Temperature != 17.5 {
		t.Errorf("slices[0].Temperature = %v, want 17.5", slices[0].Temperature)
	}
	if slices[0].RelativeHumidity != 68 {
		t.Errorf("slices[0].RelativeHumidity = %d, want 68", slices[0].RelativeHumidity)
	}
	if slices[0].PrecipitationProbability != 0 {
		t.Errorf("slices[0].PrecipitationProbability = %d, want 0", slices[0].PrecipitationProbability)
	}
}

func TestAirQualityParsesResponse(t *testing.T) {
	airHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, airQualityJSON)
	})
	c, cleanup := newGeoTestClient(http.NotFoundHandler(), http.NotFoundHandler(), airHandler)
	defer cleanup()

	aq, err := c.AirQualityAt(context.Background(), 48.85341, 2.3488)
	if err != nil {
		t.Fatal(err)
	}
	if aq.PM10 != 9.6 {
		t.Errorf("PM10 = %v, want 9.6", aq.PM10)
	}
	if aq.PM25 != 5.9 {
		t.Errorf("PM25 = %v, want 5.9", aq.PM25)
	}
	if aq.EuroAQI != 22.0 {
		t.Errorf("EuroAQI = %v, want 22.0", aq.EuroAQI)
	}
}

func TestWeatherCodeDesc(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "Clear sky"},
		{1, "Mainly clear"},
		{2, "Partly cloudy"},
		{3, "Overcast"},
		{45, "Fog"},
		{61, "Slight rain"},
		{95, "Thunderstorm"},
	}
	for _, tc := range cases {
		got := openmeteo.WeatherCodeDesc(tc.code)
		if got != tc.want {
			t.Errorf("WeatherCodeDesc(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}
