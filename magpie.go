/* A small Go program that will stuff data into your MQTT broker, data comes
 * from publically available sources and is fetched and written every set
 * interval. Longer intervals are used for non-often-changing-data (such as
 * seasons).
 *
 * This program will exit on any error, so be sure to run it in an init system
 * or other process manager.
 *
 * This program can also be ran through the use of containers, use either
 * `docker` or `podman`: `podman run -e MQTT_HOST="tcp://127.0.0.1:1883" ghcr.io/petspalace/magpie`
 *
 * Available sources:
 * - Current daylight, requires `DAYLIGHT_TOPIC`, `DAYLIGHT_LATITUDE`, and
 *   `DAYLIGHT_LONGITUDE` to be passed in the environment.
 * - Current season, requires `SEASON_TOPIC` to be  passed in the environment.
 * - Current day phase, requires `DAYPHASE_TOPIC` to be  passed in the
 *   environment.
 *
 * Sources are enabled when their respsective `_TOPIC` environment variables
 * are present. If a source is enabled and requires more configuration the
 * the program will exit if not provided.
 *
 * Bug reports, feature requests can be filed at this projects homepage which
 * you can find at https://github.com/petspalace/magpie
 *
 * This program was made by:
 * - Simon de Vlieger <cmdr@supakeen.com>
 *
 * This program is licensed under the MIT license:
 *
 * Copyright 2022 Simon de Vlieger
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to
 * deal in the Software without restriction, including without limitation the
 * rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
 * DEALINGS IN THE SOFTWARE.
 */

package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

var logger = log.New(os.Stderr, "", log.LstdFlags)

/* Message passed along between *Loop and MessageLoop through a channel,
 * *Loop determines the data and where it goes. */
type MqttCronMessage struct {
	Topic   string
	Payload string
	Retain  bool
}

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

/* Listens on a channel to submit messages to MQTT. */
func MessageLoop(c mqtt.Client, ch chan MqttCronMessage, prefix string) {
	for m := range ch {
		topic := fmt.Sprintf("%s/%s", prefix, m.Topic)

		if token := c.Publish(topic, 0, m.Retain, m.Payload); token.Wait() && token.Error() != nil {
			logger.Fatalln("MessageLoop could not publish message.")
		}

		logger.Printf("MessageLoop published topic='%s',payload='%s'\n", topic, m.Payload)
	}
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
		logger.Fatalln("WeatherAPICall could not communicate with the `buienradar.nl` domain.")
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		logger.Fatalln("WeatherAPICall could not read the response.")
	}

	var apiResult WeatherAPIResult

	if err := xml.Unmarshal(body, &apiResult); err != nil {
		logger.Fatalln("WeatherAPICall could not parse the response.")
	}

	return apiResult.Stations
}

func WeatherLoop(ch chan MqttCronMessage) {
	topicFromEnv, topicExists := os.LookupEnv("WEATHER_TOPIC")
	regionFromEnv, regionExists := os.LookupEnv("WEATHER_REGION")

	if !topicExists {
		logger.Println("WeatherLoop needs `WEATHER_TOPIC` set in the environment, disabled.")
		return
	}

	if !regionExists {
		logger.Println("WeatherLoop needs `WEATHER_REGION` set in the environment, disabled.")
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

/* Call the `sunrise-sunset.org` API and deserialize the result. */
func DayLightAPICall(apiUrl string) DayLightAPIData {
	var err error
	var res *http.Response

	if res, err = http.Get(apiUrl); err != nil {
		logger.Fatalln("DaylightAPICall could not communicate with the `api.sunrise-sunset.org` domain.")
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		logger.Fatalln("DaylightAPICall could not read the response.")
	}

	var apiResult DayLightAPIResult

	if err := json.Unmarshal(body, &apiResult); err != nil {
		logger.Fatalln("DaylightAPICall could not parse the response.")
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
		logger.Fatalln("DayLightLoop needs both `DAYLIGHT_LATITUDE` and `DAYLIGHT_LONGITUDE` set in the environment.")
	}

	var err error
	var lat float64
	var lon float64

	if lat, err = strconv.ParseFloat(latFromEnv, 32); err != nil {
		logger.Fatalf("DayLightLoop could not parse environment variable `DAYLIGHT_LATITUDE='%s'` as float.\n", latFromEnv)
	}

	if lon, err = strconv.ParseFloat(lonFromEnv, 32); err != nil {
		logger.Fatalf("DayLightLoop could not parse environment variable `DAYLIGHT_LONGITUDE='%s'` as float.\n", lonFromEnv)
	}

	logger.Print("DayLightLoop enabled.\n")

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

/* A loop that waits between submitting the current season to the
 * topic defined in the environment as `SEASON_TOPIC`. */
func SeasonLoop(ch chan MqttCronMessage) {
	topicFromEnv, topicExists := os.LookupEnv("SEASON_TOPIC")

	if !topicExists {
		logger.Println("SeasonLoop needs `SEASON_TOPIC` set in the environment, disabled.")
		return
	}

	logger.Println("SeasonLoop enabled.")

	for {
		var season string
		now := time.Now().UTC()

		if now.Month() < 3 {
			season = "winter"
		} else if now.Month() < 6 {
			season = "spring"
		} else if now.Month() < 9 {
			season = "summer"
		} else if now.Month() < 12 {
			season = "fall"
		} else {
			season = "winter"
		}

		ch <- MqttCronMessage{Retain: true, Topic: topicFromEnv, Payload: fmt.Sprintf("%s", season)}

		time.Sleep(1 * time.Hour)
	}
}

/* A loop that waits between submitting the current phase of the day
 * to the topic defined in the environment as `DAYPHASE_TOPIC`. */
func DayPhaseLoop(ch chan MqttCronMessage) {
	topicFromEnv, topicExists := os.LookupEnv("DAYPHASE_TOPIC")

	if !topicExists {
		logger.Println("DayPhaseLoop needs `DAYPHASE_TOPIC` set in the environment, disabled.")
		return
	}

	logger.Println("DayPhaseLoop enabled.")

	for {
		var dayphase string
		now := time.Now().UTC()

		if now.Hour() < 6 {
			dayphase = "night"
		} else if now.Hour() < 12 {
			dayphase = "morning"
		} else if now.Month() < 18 {
			dayphase = "afternoon"
		} else {
			dayphase = "evening"
		}

		ch <- MqttCronMessage{Retain: true, Topic: topicFromEnv, Payload: fmt.Sprintf("dayphase value=%s", dayphase)}

		time.Sleep(1 * time.Minute)
	}
}

func main() {
	ch := make(chan MqttCronMessage)

	hostFromEnv, hostExists := os.LookupEnv("MQTT_HOST")

	if !hostExists {
		logger.Fatalln("magpie needs `MQTT_HOST` set in the environment to a value such as `tcp://127.0.0.1:1883`.")
	}

	prefixFromEnv, prefixExists := os.LookupEnv("MQTT_PREFIX")

	if !prefixExists {
		logger.Println("`MQTT_PREFIX` undefined using default `home.arpa`-prefix.")
		prefixFromEnv = "/home.arpa"
	} else {
		logger.Printf("`MQTT_PREFIX` set to `%s`.\n", prefixFromEnv)
	}

	opts := mqtt.NewClientOptions().AddBroker(hostFromEnv).SetClientID("magpie")
	opts.SetKeepAlive(2 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		logger.Panic(token.Error())
	}

	go DayLightLoop(ch)
	go DayPhaseLoop(ch)
	go SeasonLoop(ch)
	go WeatherLoop(ch)

	MessageLoop(c, ch, prefixFromEnv)

	c.Disconnect(250)

	time.Sleep(1 * time.Second)
}

// SPDX-License-Identifier: MIT
// vim: ts=4 sw=4 noet
