package magpie

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type WeatherAPIStationData struct {
	Region string `xml:"regio,attr"`
	Name   string `xml:",chardata"`
}

type WeatherAPIData struct {
	Code              string                `xml:"stationcode"`
	Station           WeatherAPIStationData `xml:"stationnaam"`
	Lat               string                `xml:"lat"`
	Lon               string                `xml:"lon"`
	Humidity          string                `xml:"luchtvochtigheid"`
	TemperatureGround string                `xml:"temperatuurGC"`
	Temperature10cm   string                `xml:"temperatuur10cm"`
	WindSpeed         string                `xml:"windsnelheidMS"`
	GustSpeed         string                `xml:"windstotenMS"`
	AirPressure       string                `xml:"luchtdruk"`
	SightRange        string                `xml:"zichtmeters"`
	Rain              string                `xml:"regenMMPU"`
}

type WeatherAPIResult struct {
	XMLName  xml.Name         `xml:"buienradarnl"`
	Stations []WeatherAPIData `xml:"weergegevens>actueel_weer>weerstations>weerstation"`
}

/* The `buienradar.nl` API returns `-` when a value is not available, we convert
 * to empty string and check it later when queueing messages. */
func WeatherAPINormalizeValue(value string) string {
	if value == "-" {
		return ""
	} else {
		return value
	}
}

/* Call the `buienradar.nl` API and return the array of station data. */
func WeatherAPICall(apiUrl string) []WeatherAPIData {
	var err error
	var res *http.Response

	if res, err = http.Get(apiUrl); err != nil {
		log.Fatalln("WeatherAPICall could not communicate with the `buienradar.nl` domain.")
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		log.Fatalln("WeatherAPICall could not read the response.")
	}

	var apiResult WeatherAPIResult

	if err := xml.Unmarshal(body, &apiResult); err != nil {
		log.Fatalln("WeatherAPICall could not parse the response.")
	}

	return apiResult.Stations
}

func WeatherLoop(ch chan MqttCronMessage) {
	topicFromEnv, topicExists := os.LookupEnv("WEATHER_TOPIC")
	regionFromEnv, regionExists := os.LookupEnv("WEATHER_REGION")

	if !topicExists {
		log.Println("WeatherLoop needs `WEATHER_TOPIC` set in the environment, disabled.")
		return
	}

	if !regionExists {
		log.Println("WeatherLoop needs `WEATHER_REGION` set in the environment, disabled.")
		return
	}

	for {
		for _, location := range WeatherAPICall("https://data.buienradar.nl/1.0/feed/xml") {
			var msgs []string
			var tpcs []string

			regionName := strings.Replace(strings.ToLower(location.Station.Region), " ", "-", -1)

			if regionName != regionFromEnv {
				continue
			}

			if len(WeatherAPINormalizeValue(location.Humidity)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "humidity"))
				msgs = append(msgs, fmt.Sprintf("%s", location.Humidity))
			}

			if len(WeatherAPINormalizeValue(location.TemperatureGround)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "temperature.ground"))
				msgs = append(msgs, fmt.Sprintf("%s", location.TemperatureGround))
			}

			if len(WeatherAPINormalizeValue(location.Temperature10cm)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "temperature.10cm"))
				msgs = append(msgs, fmt.Sprintf("%s", location.Temperature10cm))
			}

			if len(WeatherAPINormalizeValue(location.WindSpeed)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "wind"))
				msgs = append(msgs, fmt.Sprintf("%s", location.WindSpeed))
			}

			if len(WeatherAPINormalizeValue(location.GustSpeed)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "gust"))
				msgs = append(msgs, fmt.Sprintf("%s", location.GustSpeed))
			}

			if len(WeatherAPINormalizeValue(location.AirPressure)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "pressure"))
				msgs = append(msgs, fmt.Sprintf("%s", location.AirPressure))
			}

			if len(WeatherAPINormalizeValue(location.Rain)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "rain"))
				msgs = append(msgs, fmt.Sprintf("%s", location.Rain))
			}

			if len(WeatherAPINormalizeValue(location.SightRange)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "sight"))
				msgs = append(msgs, fmt.Sprintf("%s", location.SightRange))
			}

			for idx, msg := range msgs {
				ch <- MqttCronMessage{Retain: false, Topic: tpcs[idx], Payload: msg}
			}
		}

		time.Sleep(5 * time.Minute)
	}
}
