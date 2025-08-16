package api

// JMA API implementation for fetching weather forecasts from the Japan Meteorological Agency

import (
	"fmt"
)

// AreaCode represents the code for a geographical area in Japan
type AreaCode string

// AreaCodeMap maps area names to their respective area codes
var AreaCodeMap = map[string]AreaCode{
	"宗谷地方":       "011000",
	"上川・留萌地方":    "012000",
	"石狩・空知・後志地方": "016000",
	"網走・北見・紋別地方": "013000",
	"釧路・根室地方":    "014100",
	"胆振・日高地方":    "015000",
	"渡島・檜山地方":    "017000",
	"青森県":        "020000",
	"秋田県":        "050000",
	"岩手県":        "030000",
	"宮城県":        "040000",
	"山形県":        "060000",
	"福島県":        "070000",
	"茨城県":        "080000",
	"栃木県":        "090000",
	"群馬県":        "100000",
	"埼玉県":        "110000",
	"東京都":        "130000",
	"千葉県":        "120000",
	"神奈川県":       "140000",
	"長野県":        "200000",
	"山梨県":        "190000",
	"静岡県":        "220000",
	"愛知県":        "230000",
	"岐阜県":        "210000",
	"三重県":        "240000",
	"新潟県":        "150000",
	"富山県":        "160000",
	"石川県":        "170000",
	"福井県":        "180000",
	"滋賀県":        "250000",
	"京都府":        "260000",
	"大阪府":        "270000",
	"兵庫県":        "280000",
	"奈良県":        "290000",
	"和歌山県":       "300000",
	"岡山県":        "330000",
	"広島県":        "340000",
	"島根県":        "320000",
	"鳥取県":        "310000",
	"徳島県":        "360000",
	"香川県":        "370000",
	"愛媛県":        "380000",
	"高知県":        "390000",
	"山口県":        "350000",
	"福岡県":        "400000",
	"大分県":        "440000",
	"長崎県":        "420000",
	"佐賀県":        "410000",
	"熊本県":        "430000",
	"宮崎県":        "450000",
	"鹿児島県":       "460100",
	"沖縄本島地方":     "471000",
	"大東島地方":      "472000",
	"宮古島地方":      "473000",
	"八重山地方":      "474000",
}

// RawArea represents an area with weather forecasts in raw API response
type RawArea struct {
	Area struct {
		Name string   `json:"name"`
		Code AreaCode `json:"code"`
	} `json:"area"`
	Weathers []string `json:"weathers"`
	Winds    []string `json:"winds"`
	Waves    []string `json:"waves,omitempty"`
	Temps    []string `json:"temps,omitempty"`
}

// RawTimeSeriesItem represents a time series entry in the raw API response
type RawTimeSeriesItem struct {
	TimeDefines []string  `json:"timeDefines"`
	Areas       []RawArea `json:"areas"`
}

// RawWeatherForecast represents the raw API response from JMA
type RawWeatherForecast struct {
	PublishingOffice string              `json:"publishingOffice"`
	ReportDateTime   string              `json:"reportDateTime"`
	TimeSeries       []RawTimeSeriesItem `json:"timeSeries"`
}

// WeatherInfo holds weather details for a specific time
type WeatherInfo struct {
	Time    string  `json:"time"`
	Weather *string `json:"weather,omitempty"`
	Wind    *string `json:"wind,omitempty"`
	Wave    *string `json:"wave,omitempty"`
}

// AreaForecast contains weather forecasts for a specific area
type AreaForecast struct {
	AreaName string        `json:"areaName"`
	AreaCode AreaCode      `json:"areaCode"`
	Weathers []WeatherInfo `json:"weathers"`
}

// TemperatureInfo holds temperature details for a specific time
type TemperatureInfo struct {
	Time        string  `json:"time"`
	Temperature *string `json:"temperature,omitempty"`
}

// TemperatureForecast contains temperature forecasts for a specific area
type TemperatureForecast struct {
	AreaName     string            `json:"areaName"`
	Temperatures []TemperatureInfo `json:"temperatures,omitempty"`
}

// WeatherForecast contains all weather and temperature forecasts
type WeatherForecast struct {
	ReportDateTime       string                `json:"reportDateTime"`
	AreaForecasts        []AreaForecast        `json:"areaForecasts"`
	TemperatureForecasts []TemperatureForecast `json:"temperatureForecasts"`
}

// JmaAPI provides methods to interact with Japan Meteorological Agency API
type JmaAPI interface {
	GetAreaCodeMap() map[string]AreaCode
	GetWeatherForecast(code AreaCode) (*WeatherForecast, error)
}

// JmaApi provides methods to interact with the Japan Meteorological Agency API
type JmaApi struct {
	jsonApi *JsonApi
}

// NewJmaApi creates a new JMA API client
func NewJmaApi() *JmaApi {
	return &JmaApi{
		jsonApi: NewJsonApi("https://www.jma.go.jp/bosai/forecast/data", JsonApiCustom{}),
	}
}

// GetAreaCodeMap returns the map of area names to area codes
func (j *JmaApi) GetAreaCodeMap() map[string]AreaCode {
	return AreaCodeMap
}

// GetWeatherForecast fetches the weather forecast for a specific area code
func (j *JmaApi) GetWeatherForecast(code AreaCode) (*WeatherForecast, error) {
	var rawForecasts []RawWeatherForecast
	err := j.jsonApi.Get(fmt.Sprintf("/forecast/%s.json", code), &rawForecasts)
	if err != nil {
		return nil, fmt.Errorf("failed to get weather forecast: %w", err)
	}

	// rawForecasts[0] = weather forecast
	// rawForecasts[1] = ?
	if len(rawForecasts) < 1 {
		return nil, fmt.Errorf("unexpected response format: empty forecast array")
	}

	rawForecast := rawForecasts[0]

	// Ensure we have necessary time series
	if len(rawForecast.TimeSeries) < 3 {
		return nil, fmt.Errorf("unexpected response format: insufficient time series")
	}

	threeDaySeries := rawForecast.TimeSeries[0]
	temperatureSeries := rawForecast.TimeSeries[2]

	// Process area forecasts
	areaForecasts := make([]AreaForecast, 0, len(threeDaySeries.Areas))
	for _, a := range threeDaySeries.Areas {
		weatherInfos := make([]WeatherInfo, len(threeDaySeries.TimeDefines))
		for j, t := range threeDaySeries.TimeDefines {
			info := WeatherInfo{
				Time: t,
			}

			if j < len(a.Weathers) {
				weather := a.Weathers[j]
				info.Weather = &weather
			}

			if j < len(a.Winds) {
				wind := a.Winds[j]
				info.Wind = &wind
			}

			if j < len(a.Waves) {
				wave := a.Waves[j]
				info.Wave = &wave
			}

			weatherInfos[j] = info
		}

		areaForecasts = append(areaForecasts, AreaForecast{
			AreaName: a.Area.Name,
			AreaCode: a.Area.Code,
			Weathers: weatherInfos,
		})
	}

	// Process temperature forecasts
	temperatureForecasts := make([]TemperatureForecast, 0, len(temperatureSeries.Areas))
	for _, a := range temperatureSeries.Areas {
		if a.Temps == nil || len(a.Temps) == 0 {
			temperatureForecasts = append(temperatureForecasts, TemperatureForecast{
				AreaName: a.Area.Name,
			})
			continue
		}

		tempInfos := make([]TemperatureInfo, len(temperatureSeries.TimeDefines))
		for i, t := range temperatureSeries.TimeDefines {
			info := TemperatureInfo{
				Time: t,
			}

			if i < len(a.Temps) {
				temp := a.Temps[i]
				info.Temperature = &temp
			}

			tempInfos[i] = info
		}

		temperatureForecasts = append(temperatureForecasts, TemperatureForecast{
			AreaName:     a.Area.Name,
			Temperatures: tempInfos,
		})
	}

	return &WeatherForecast{
		ReportDateTime:       rawForecast.ReportDateTime,
		AreaForecasts:        areaForecasts,
		TemperatureForecasts: temperatureForecasts,
	}, nil
}
