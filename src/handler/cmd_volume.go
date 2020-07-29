package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/thingsplex/bose/model"
)

type Volume struct {
	states *model.States
}

// VolumeSet sends request to Bose to set new volume lvl
func (vol *Volume) VolumeSet(val int64, ip string, port string) (bool, error) {
	url := fmt.Sprintf("%s%s%s%s%s", "http://", ip, ":", port, "/volume")

	payload := strings.NewReader(fmt.Sprintf("<volume>%d</volume>", val))
	log.Debug(payload)

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		log.Error("ERror when setting volume: ", err)
		return false, err
	}
	req.Header.Set("Content-Type", "application/xml")
	log.Debug("New request: ", req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Error when DefaultClient.Do on VolumeSet: ", err)
		return false, err
	}
	log.Debug(resp)
	if resp.StatusCode != 200 {
		log.Error("Bad HTTP return code ", resp.StatusCode)
		return false, fmt.Errorf("%s%s", "Bad HTTP return code ", strconv.Itoa(resp.StatusCode))
	}

	return true, nil
}
