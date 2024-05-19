# magpie

Small Golang program to stuff data into MQTT on a schedule.

## installation

### container

This will start a long-running container with mqtt cron in it which runs all
available subcommands. The container is available for `amd64` and `aarch64`.

```
podman run -e MQTT_HOST="tcp://hostname:1883" ghcr.io/petspalace/magpie:latest
```

*The `podman` command can be switched out `docker` if you wish.*

## usage 

To enable sources pass their relevant environment variables.

### daylight

Puts a retained topic into MQTT which contains `yes` or `no` to indicate if it
is currently daylight.

- `DAYLIGHT_TOPIC`, the topic in MQTT to use.
- `DAYLIGHT_LATITUDE`, latitude of location for daylight.
- `DAYLIGHT_LONGITUDE`, longitude of location for daylight.

For example: `MQTT_HOST="tcp://localhost:1883" DAYLIGHT_TOPIC="/cron/daylight" DAYLIGHT_LATITUDE="52.078663" DAYLIGHT_LONGITUDE="4.288788" ./bin/magpie-linux-amd64`
to publish the daylight status for *The Hague, The Netherlands* to the `/cron/daylight` topic.

### season

Puts a retained topic into MQTT which contains `spring`, `summer`, `fall`, or
`winter`, depending on the current date.

- `SEASON_TOPIC`, the topic in MQTT to use.

### dayphase

Puts a retained topic into MQTT which contains `morning`, `afternoon`,
`evening`, or `night` depending on the current time.

- `DAYPHASE_TOPIC`, the topic in MQTT to use.
