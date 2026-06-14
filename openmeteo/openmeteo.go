// Package openmeteo is the library behind the openmeteo command line:
// the HTTP client, request shaping, and the typed data models for the Open-Meteo
// weather API (api.open-meteo.com).
//
// Open-Meteo is a free, open-source weather API providing current conditions and
// multi-day forecasts for any geographic coordinate. No API key or registration
// is required. Data is sourced from national meteorological services and refreshed
// every 15 minutes.
package openmeteo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Host is the API host this client talks to.
const Host = "api.open-meteo.com"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	GeoURL    string
	AirURL    string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://api.open-meteo.com/v1",
		GeoURL:    "https://geocoding-api.open-meteo.com/v1",
		AirURL:    "https://air-quality-api.open-meteo.com/v1",
		UserAgent: "openmeteo-cli/0.1.0 (github.com/tamnd/openmeteo-cli)",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Client talks to the Open-Meteo API over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Current fetches current weather conditions for the given coordinates.
// The returned CurrentWeather contains temperature, wind, humidity, pressure,
// and a WMO weather code with a human-readable description.
func (c *Client) Current(ctx context.Context, lat, lon float64) (*CurrentWeather, error) {
	u := fmt.Sprintf(
		"%s/forecast?latitude=%s&longitude=%s&current=temperature_2m,relative_humidity_2m,wind_speed_10m,wind_direction_10m,weathercode,surface_pressure&timezone=auto",
		c.cfg.BaseURL,
		strconv.FormatFloat(lat, 'f', -1, 64),
		strconv.FormatFloat(lon, 'f', -1, 64),
	)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp currentResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode current weather: %w", err)
	}
	cur := resp.Current
	return &CurrentWeather{
		Time:        cur.Time,
		TempC:       cur.Temperature,
		WindKph:     cur.WindSpeed,
		WindDir:     cur.WindDir,
		Humidity:    cur.Humidity,
		Pressure:    cur.Pressure,
		WeatherCode: cur.WeatherCode,
		Description: weatherDescription(cur.WeatherCode),
		Latitude:    resp.Latitude,
		Longitude:   resp.Longitude,
		Timezone:    resp.Timezone,
	}, nil
}

// Forecast fetches a daily forecast for the given coordinates.
// days must be 1–16; values ≤ 0 default to 7, values > 16 are clamped to 16.
func (c *Client) Forecast(ctx context.Context, lat, lon float64, days int) ([]DailyForecast, error) {
	if days <= 0 {
		days = 7
	}
	if days > 16 {
		days = 16
	}
	u := fmt.Sprintf(
		"%s/forecast?latitude=%s&longitude=%s&daily=temperature_2m_max,temperature_2m_min,precipitation_sum,wind_speed_10m_max,weathercode&forecast_days=%d&timezone=auto",
		c.cfg.BaseURL,
		strconv.FormatFloat(lat, 'f', -1, 64),
		strconv.FormatFloat(lon, 'f', -1, 64),
		days,
	)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp dailyResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode forecast: %w", err)
	}
	d := resp.Daily
	n := len(d.Time)
	out := make([]DailyForecast, 0, n)
	for i := 0; i < n; i++ {
		var tempMax, tempMin, precip, windMax float64
		var code int
		if i < len(d.TempMax) {
			tempMax = d.TempMax[i]
		}
		if i < len(d.TempMin) {
			tempMin = d.TempMin[i]
		}
		if i < len(d.PrecipSum) {
			precip = d.PrecipSum[i]
		}
		if i < len(d.WindMax) {
			windMax = d.WindMax[i]
		}
		if i < len(d.WeatherCode) {
			code = d.WeatherCode[i]
		}
		out = append(out, DailyForecast{
			Rank:        i + 1,
			Date:        d.Time[i],
			TempMaxC:    tempMax,
			TempMinC:    tempMin,
			PrecipMM:    precip,
			WindMaxKph:  windMax,
			WeatherCode: code,
			Description: weatherDescription(code),
		})
	}
	return out, nil
}

// Geocode resolves a city name to geographic coordinates.
// Returns up to count results ordered by population.
func (c *Client) Geocode(ctx context.Context, name string, count int) ([]GeoResult, error) {
	if count <= 0 {
		count = 5
	}
	u := fmt.Sprintf("%s/search?name=%s&count=%d", c.cfg.GeoURL, name, count)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp geoSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode geocode: %w", err)
	}
	return resp.Results, nil
}

// HourlyForecast fetches an hourly forecast for the given coordinates.
// days must be 1–16; values <= 0 default to 3, values > 16 are clamped to 16.
func (c *Client) HourlyForecast(ctx context.Context, lat, lon float64, days int) ([]HourlySlice, error) {
	if days <= 0 {
		days = 3
	}
	if days > 16 {
		days = 16
	}
	u := fmt.Sprintf(
		"%s/forecast?latitude=%s&longitude=%s&hourly=temperature_2m,relative_humidity_2m,precipitation_probability,wind_speed_10m&timezone=auto&forecast_days=%d",
		c.cfg.BaseURL,
		strconv.FormatFloat(lat, 'f', -1, 64),
		strconv.FormatFloat(lon, 'f', -1, 64),
		days,
	)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp hourlyResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode hourly forecast: %w", err)
	}
	h := resp.Hourly
	n := len(h.Time)
	out := make([]HourlySlice, 0, n)
	for i := 0; i < n; i++ {
		s := HourlySlice{Time: h.Time[i]}
		if i < len(h.Temperature) {
			s.Temperature = h.Temperature[i]
		}
		if i < len(h.RelativeHumidity) {
			s.RelativeHumidity = h.RelativeHumidity[i]
		}
		if i < len(h.PrecipitationProbability) {
			s.PrecipitationProbability = h.PrecipitationProbability[i]
		}
		if i < len(h.WindSpeed) {
			s.WindSpeed = h.WindSpeed[i]
		}
		out = append(out, s)
	}
	return out, nil
}

// AirQualityAt fetches current air quality for the given coordinates.
func (c *Client) AirQualityAt(ctx context.Context, lat, lon float64) (AirQuality, error) {
	u := fmt.Sprintf(
		"%s/air-quality?latitude=%s&longitude=%s&current=pm10,pm2_5,carbon_monoxide,nitrogen_dioxide,ozone,european_aqi",
		c.cfg.AirURL,
		strconv.FormatFloat(lat, 'f', -1, 64),
		strconv.FormatFloat(lon, 'f', -1, 64),
	)
	body, err := c.get(ctx, u)
	if err != nil {
		return AirQuality{}, err
	}
	var resp airQualityResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return AirQuality{}, fmt.Errorf("decode air quality: %w", err)
	}
	b := resp.Current
	return AirQuality{
		PM10:    b.PM10,
		PM25:    b.PM25,
		CO:      b.CO,
		NO2:     b.NO2,
		Ozone:   b.Ozone,
		EuroAQI: b.EuroAQI,
	}, nil
}

// WeatherCodeDesc maps a WMO weather interpretation code to a human description.
func WeatherCodeDesc(code int) string {
	switch code {
	case 0:
		return "Clear sky"
	case 1:
		return "Mainly clear"
	case 2:
		return "Partly cloudy"
	case 3:
		return "Overcast"
	case 45:
		return "Fog"
	case 48:
		return "Depositing rime fog"
	case 51:
		return "Light drizzle"
	case 53:
		return "Moderate drizzle"
	case 55:
		return "Dense drizzle"
	case 61:
		return "Slight rain"
	case 63:
		return "Moderate rain"
	case 65:
		return "Heavy rain"
	case 71:
		return "Slight snow"
	case 73:
		return "Moderate snow"
	case 75:
		return "Heavy snow"
	case 80:
		return "Slight rain showers"
	case 81:
		return "Moderate rain showers"
	case 82:
		return "Violent rain showers"
	case 95:
		return "Thunderstorm"
	default:
		return fmt.Sprintf("Unknown (code %d)", code)
	}
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	return b, err != nil, err
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}

// weatherDescription maps a WMO 4677 weather code to a human-readable string.
func weatherDescription(code int) string {
	switch {
	case code == 0:
		return "Clear sky"
	case code <= 3:
		return "Partly cloudy"
	case code <= 49:
		return "Foggy"
	case code <= 69:
		return "Rain"
	case code <= 79:
		return "Snow"
	case code <= 99:
		return "Thunderstorm"
	default:
		return "Unknown"
	}
}

// --- internal API response types ---

type currentResponse struct {
	Latitude  float64      `json:"latitude"`
	Longitude float64      `json:"longitude"`
	Timezone  string       `json:"timezone"`
	Current   currentBlock `json:"current"`
}

type currentBlock struct {
	Time        string  `json:"time"`
	Temperature float64 `json:"temperature_2m"`
	Humidity    int     `json:"relative_humidity_2m"`
	WindSpeed   float64 `json:"wind_speed_10m"`
	WindDir     int     `json:"wind_direction_10m"`
	WeatherCode int     `json:"weathercode"`
	Pressure    float64 `json:"surface_pressure"`
}

type dailyResponse struct {
	Latitude  float64    `json:"latitude"`
	Longitude float64    `json:"longitude"`
	Timezone  string     `json:"timezone"`
	Daily     dailyBlock `json:"daily"`
}

type dailyBlock struct {
	Time        []string  `json:"time"`
	TempMax     []float64 `json:"temperature_2m_max"`
	TempMin     []float64 `json:"temperature_2m_min"`
	PrecipSum   []float64 `json:"precipitation_sum"`
	WindMax     []float64 `json:"wind_speed_10m_max"`
	WeatherCode []int     `json:"weathercode"`
}
