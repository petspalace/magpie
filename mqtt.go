package magpie

/* Message passed along between *Loop and MessageLoop through a channel,
 * *Loop determines the data and where it goes. */
type MqttCronMessage struct {
	Topic   string
	Payload string
	Retain  bool
}
