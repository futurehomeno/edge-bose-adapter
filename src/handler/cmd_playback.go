package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/futurehomeno/edge-bose-adapter/model"

	log "github.com/sirupsen/logrus"
)

// Playback attributes
type Playback struct {
	states *model.States
}

// PlaybackSet sends request to Sonos to play, pause or skip
func (pb *Playback) PlaybackSet(val string, ip string, port string) (bool, error) {
	// change toggle_play_pause to PLAY_PAUSE, next_track to NEXT_TRACK and previous_track to PREV_TRACK
	log.Debug("THIS IS THE VALUE BEFORE: ", val)
	if val == "toggle_play_pause" {
		val = "PLAY_PAUSE"
	} else if val == "next_track" {
		val = "NEXT_TRACK"
	} else if val == "previous_track" {
		val = "PREV_TRACK"
	} else if val == "play" {
		val = "PLAY"
	} else if val == "pause" {
		val = "PAUSE"
	}
	log.Debug("THIS IS THE VALUE AFTER: ", val)

	url := fmt.Sprintf("%s%s%s%s%s", "http://", ip, ":", port, "/key")

	payload := strings.NewReader(fmt.Sprintf("<key state=\"press\" sender=\"Gabbo\">%s</key>", val))
	log.Debug(payload)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		log.Error("Error when setting playback: ", err)
		return false, err
	}
	req.Header.Set("Content-Type", "application/xml")
	log.Debug("New request: ", req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Error when DefaultClient.Do on PlaybackSet: ", err)
		return false, err
	}
	log.Debug(resp)
	if resp.StatusCode != 200 {
		log.Error("Bad HTTP return code ", resp.StatusCode)
		return false, fmt.Errorf("%s%s", "Bad HTTP return code ", strconv.Itoa(resp.StatusCode))
	}

	return true, nil
}

func (pb *Playback) ModeSet(val map[string]bool, ip string, port string) (bool, error) {
	url := fmt.Sprintf("%s%s%s%s%s", "http://", ip, ":", port, "/key")
	// change to correct values
	var values []string
	log.Debug("values: ", values)
	if shuffle, ok := val["shuffle"]; ok {
		if shuffle {
			values = append(values, "SHUFFLE_ON")
			// values["shuffle"] = "SHUFFLE_ON"
		} else {
			values = append(values, "SHUFFLE_OFF")
			// values["shuffle"] = "SHUFFLE_OFF"
		}
	}
	if repeatOne, ok := val["repeat_one"]; ok {
		if repeatOne {
			values = append(values, "REPEAT_ONE")
			// values["repeat"] = "REPEAT_ONE"
		} else {
			values = append(values, "REPEAT_OFF")
			// values["repeat"] = "REPEAT_OFF"
		}
	}
	if repeatAll, ok := val["repeat"]; ok {
		if repeatAll {
			values = append(values, "REPEAT_ALL")
			// values["repeat"] = "REPEAT_ALL"
		} else {
			values = append(values, "REPEAT_OFF")
			// values["repeat"] = "REPEAT_OFF"
		}
	}
	log.Debug("values after: ", values)
	for i := 0; i < len(values); i++ {
		payload := strings.NewReader(fmt.Sprintf("<key state=\"press\" sender=\"Gabbo\">%s</key>", values[i]))

		req, err := http.NewRequest("POST", url, payload)
		if err != nil {
			log.Error("Error when setting mode: ", err)
			return false, err
		}
		req.Header.Set("Content-Type", "application/xml")
		log.Debug("New Request: ", req)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Error("Error when DefaultClient.Do on ModeSet: ", err)
			return false, err
		}
		log.Debug(resp)
		if resp.StatusCode != 200 {
			log.Error("Bad HTTP return code ", resp.StatusCode)
			return false, fmt.Errorf("%s%s", "Bad HTTP return code ", strconv.Itoa(resp.StatusCode))
		}
	}
	return true, nil
}

func (pb *Playback) WakeUp(ip string, port string) (bool, error) {
	url := fmt.Sprintf("%s%s%s%s%s", "http://", ip, ":", port, "/key")
	payload := strings.NewReader(fmt.Sprintf("<key state=\"press\" sender=\"Gabbo\">%s</key>", "POWER"))
	req, err := http.NewRequest("POST", url, payload)
	req.Header.Set("Content-Type", "application/xml")
	if err != nil {
		log.Error("Error when waking device: ", err)
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Error when DefaultClient.Do on WakeUp: ", err)
		return false, err
	}
	if resp.StatusCode != 200 {
		log.Error("Bad HTTP return code ", resp.StatusCode)
		return false, fmt.Errorf("%s%s", "Bad HTTP return code ", strconv.Itoa(resp.StatusCode))
	}

	payload = strings.NewReader(fmt.Sprintf("<key state=\"release\" sender=\"Gabbo\">%s</key>", "POWER"))
	req, err = http.NewRequest("POST", url, payload)
	if err != nil {
		log.Error("Error when waking device: ", err)
		return false, err
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Error when DefaultClient.Do on WakeUp: ", err)
		return false, err
	}
	if resp.StatusCode != 200 {
		log.Error("Bad HTTP return code ", resp.StatusCode)
		return false, fmt.Errorf("%s%s", "Bad HTTP return code ", strconv.Itoa(resp.StatusCode))
	}
	return true, nil
}
