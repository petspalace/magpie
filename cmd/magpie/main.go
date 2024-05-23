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
	"fmt"
	"log"
	"os"
	"time"

	"github.com/eclipse/paho.mqtt.golang"

	"github.com/petspalace/magpie"
)

var logger = log.New(os.Stderr, "", log.LstdFlags)

/* Listens on a channel to submit messages to MQTT. */
func MessageLoop(c mqtt.Client, ch chan magpie.MqttCronMessage, prefix string) {
	for m := range ch {
		topic := fmt.Sprintf("%s/%s", prefix, m.Topic)

		if token := c.Publish(topic, 0, m.Retain, m.Payload); token.Wait() && token.Error() != nil {
			logger.Fatalln("MessageLoop could not publish message.")
		}

		logger.Printf("MessageLoop published topic='%s',payload='%s'\n", topic, m.Payload)
	}
}


func main() {
	ch := make(chan magpie.MqttCronMessage)

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

	go magpie.DayLightLoop(ch)
	go magpie.DayPhaseLoop(ch)
	go magpie.SeasonLoop(ch)
	go magpie.WeatherLoop(ch)

	MessageLoop(c, ch, prefixFromEnv)

	c.Disconnect(250)

	time.Sleep(1 * time.Second)
}

// SPDX-License-Identifier: MIT
// vim: ts=4 sw=4 noet
