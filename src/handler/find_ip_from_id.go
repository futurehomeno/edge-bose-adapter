package handler

import (
	"fmt"

	"github.com/thingsplex/bose/bose-api"
)

// GetIPFromID finds the IP address of the player
func GetIPFromID(deviceID string, Players []bose.Player) (string, error) {
	for i := 0; i < len(Players); i++ {
		if deviceID == Players[i].DeviceID {
			return Players[i].NetworkInfo[0].IpAddress, nil
		}
	}
	err := fmt.Errorf("Could not find device with given deviceID")
	return "", err
}
