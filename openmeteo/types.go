package openmeteo

// CurrentWeather holds the current weather conditions for a location.
type CurrentWeather struct {
	Time        string  `json:"time"`
	TempC       float64 `json:"temp_c"`
	WindKph     float64 `json:"wind_kph"`
	WindDir     int     `json:"wind_dir"`     // degrees (0 if not requested)
	Humidity    int     `json:"humidity"`     // % (0 if not requested)
	Pressure    float64 `json:"pressure_hpa"` // hPa
	WeatherCode int     `json:"weather_code"` // WMO code
	Description string  `json:"description"`  // from WMO code
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Timezone    string  `json:"timezone"`
}

// DailyForecast holds the forecast for a single day.
type DailyForecast struct {
	Rank        int     `json:"rank"`
	Date        string  `json:"date"`
	TempMaxC    float64 `json:"temp_max_c"`
	TempMinC    float64 `json:"temp_min_c"`
	PrecipMM    float64 `json:"precip_mm"`
	WindMaxKph  float64 `json:"wind_max_kph"`
	WeatherCode int     `json:"weather_code"`
	Description string  `json:"description"`
}

// GeoResult is one result from the geocoding API.
type GeoResult struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Timezone    string  `json:"timezone"`
}

// HourlySlice is one hour's weather data after transposing the parallel arrays.
type HourlySlice struct {
	Time                     string  `json:"time"`
	Temperature              float64 `json:"temperature_2m"`
	RelativeHumidity         int     `json:"relative_humidity_2m"`
	PrecipitationProbability int     `json:"precipitation_probability"`
	WindSpeed                float64 `json:"wind_speed_10m"`
}

// AirQuality holds current air quality measurements.
type AirQuality struct {
	PM10    float64 `json:"pm10"`
	PM25    float64 `json:"pm2_5"`
	CO      float64 `json:"carbon_monoxide"`
	NO2     float64 `json:"nitrogen_dioxide"`
	Ozone   float64 `json:"ozone"`
	EuroAQI float64 `json:"european_aqi"`
}

// --- internal decode types ---

type geoSearchResponse struct {
	Results []GeoResult `json:"results"`
}

type hourlyResponse struct {
	Timezone string      `json:"timezone"`
	Hourly   hourlyBlock `json:"hourly"`
}

type hourlyBlock struct {
	Time                     []string  `json:"time"`
	Temperature              []float64 `json:"temperature_2m"`
	RelativeHumidity         []int     `json:"relative_humidity_2m"`
	PrecipitationProbability []int     `json:"precipitation_probability"`
	WindSpeed                []float64 `json:"wind_speed_10m"`
}

type airQualityResponse struct {
	Current airQualityBlock `json:"current"`
}

type airQualityBlock struct {
	PM10    float64 `json:"pm10"`
	PM25    float64 `json:"pm2_5"`
	CO      float64 `json:"carbon_monoxide"`
	NO2     float64 `json:"nitrogen_dioxide"`
	Ozone   float64 `json:"ozone"`
	EuroAQI float64 `json:"european_aqi"`
}
