package magpie

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

/* Result data from the `sunrise-sunset.org` API. */
type DayLightAPIData struct {
	Sunrise                   time.Time `json:"sunrise"`
	Sunset                    time.Time `json:"sunset"`
	SolarNoon                 time.Time `json:"solar_noon"`
	DayLength                 int       `json:"day_length"`
	CivilTwilightBegin        time.Time `json:"civil_twilight_begin"`
	CivilTwilightEnd          time.Time `json:"civil_twilight_end"`
	NauticalTwilightBegin     time.Time `json:"nautical_twilight_begin"`
	NauticalTwilightEnd       time.Time `json:"nautical_twilight_end"`
	AstronomicalTwilightBegin time.Time `json:"astronomical_twilight_begin"`
	AstronomicalTwilightEnd   time.Time `json:"astronomical_twilight_end"`
}

/* Result from the `sunrise-sunset.org` API. */
type DayLightAPIResult struct {
	Status  string          `json:"status"`
	Results DayLightAPIData `json:"results"`
}

/* Call the `sunrise-sunset.org` API and deserialize the result. */
func DayLightAPICall(apiUrl string) DayLightAPIData {
	var err error
	var res *http.Response

	if res, err = http.Get(apiUrl); err != nil {
		log.Fatalln("DaylightAPICall could not communicate with the `api.sunrise-sunset.org` domain.")
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		log.Fatalln("DaylightAPICall could not read the response.")
	}

	var apiResult DayLightAPIResult

	if err := json.Unmarshal(body, &apiResult); err != nil {
		log.Fatalln("DaylightAPICall could not parse the response.")
	}

	return apiResult.Results
}

/* A loop that waits between calls to the `sunrise-sunset.org` API
 * and submits the current daylight status to the topic given in the
 * environment variable `DAYLIGHT_TOPIC`. */
func DayLightLoop(ch chan MqttCronMessage) {
	topicFromEnv, topicExists := os.LookupEnv("DAYLIGHT_TOPIC")

	if !topicExists {
		log.Println("DayLightLoop needs `DAYLIGHT_TOPIC` set in the environment, disabled.")
		return
	}

	latFromEnv, latExists := os.LookupEnv("DAYLIGHT_LATITUDE")
	lonFromEnv, lonExists := os.LookupEnv("DAYLIGHT_LONGITUDE")

	if !latExists || !lonExists {
		log.Fatalln("DayLightLoop needs both `DAYLIGHT_LATITUDE` and `DAYLIGHT_LONGITUDE` set in the environment.")
	}

	var err error
	var lat float64
	var lon float64

	if lat, err = strconv.ParseFloat(latFromEnv, 32); err != nil {
		log.Fatalf("DayLightLoop could not parse environment variable `DAYLIGHT_LATITUDE='%s'` as float.\n", latFromEnv)
	}

	if lon, err = strconv.ParseFloat(lonFromEnv, 32); err != nil {
		log.Fatalf("DayLightLoop could not parse environment variable `DAYLIGHT_LONGITUDE='%s'` as float.\n", lonFromEnv)
	}

	log.Print("DayLightLoop enabled.\n")

	apiUrl := fmt.Sprintf("https://api.sunrise-sunset.org/json?lat=%f&lng=%f&date=today&formatted=0", lat, lon)

	for {
		apiResult := DayLightAPICall(apiUrl)

		var isDayTime string
		now := time.Now().UTC()

		if now.Before(apiResult.Sunrise.UTC()) || now.After(apiResult.Sunset.UTC()) {
			isDayTime = "no"
		} else {
			isDayTime = "yes"
		}

		ch <- MqttCronMessage{Retain: true, Topic: topicFromEnv, Payload: fmt.Sprintf("%s", isDayTime)}

		time.Sleep(1 * time.Hour)
	}
}
