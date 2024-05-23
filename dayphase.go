package magpie

import (
	"fmt"
	"log"
	"os"
	"time"
)

/* A loop that waits between submitting the current phase of the day
 * to the topic defined in the environment as `DAYPHASE_TOPIC`. */
func DayPhaseLoop(ch chan MqttCronMessage) {
	topicFromEnv, topicExists := os.LookupEnv("DAYPHASE_TOPIC")

	if !topicExists {
		log.Println("DayPhaseLoop needs `DAYPHASE_TOPIC` set in the environment, disabled.")
		return
	}

	log.Println("DayPhaseLoop enabled.")

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
