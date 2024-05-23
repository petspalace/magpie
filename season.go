package magpie

import (
	"fmt"
	"log"
	"os"
	"time"
)

/* A loop that waits between submitting the current season to the
 * topic defined in the environment as `SEASON_TOPIC`. */
func SeasonLoop(ch chan MqttCronMessage) {
	topicFromEnv, topicExists := os.LookupEnv("SEASON_TOPIC")

	if !topicExists {
		log.Println("SeasonLoop needs `SEASON_TOPIC` set in the environment, disabled.")
		return
	}

	log.Println("SeasonLoop enabled.")

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
