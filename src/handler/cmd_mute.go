package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/futurehomeno/edge-bose-adapter/model"
	log "github.com/sirupsen/logrus"
)

// Mute attributes
type Mute struct {
	states *model.States
}

// MuteSet sends request to Bose to mute or unmute speaker
func (mute *Mute) MuteSet(ip string, port string) (bool, error) {
	url := fmt.Sprintf("%s%s%s%s%s", "http://", ip, ":", port, "/key")

	payload := strings.NewReader("<key state=\"press\" sender=\"Gabbo\">MUTE</key>")

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		log.Error("Error when setting mute: ", err)
		return false, err
	}
	req.Header.Set("Content-Type", "application/xml")
	log.Debug("New request: ", req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Error when DefaultClient.Do on MuteSet: ", err)
		return false, err
	}
	log.Debug(resp)
	if resp.StatusCode != 200 {
		log.Error("Bad HTTP return code ", resp.StatusCode)
		return false, fmt.Errorf("%s%s", "Bad HTTP return code ", strconv.Itoa(resp.StatusCode))
	}

	return true, nil
}
