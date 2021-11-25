package model

import (
	"fmt"

	"github.com/futurehomeno/edge-bose-adapter/bose-api"

	"github.com/futurehomeno/fimpgo/fimptype"
)

// MakeInclusionReport makes inclusion report for player with id given in parameter
func MakeInclusionReport(Player bose.Player) fimptype.ThingInclusionReport {
	// var err error

	var name, manufacturer string
	var deviceAddr string
	services := []fimptype.Service{}

	mediaPlayerInterfaces := []fimptype.Interface{{
		Type:      "in",
		MsgType:   "cmd.playback.set",
		ValueType: "string",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.playback.get_report",
		ValueType: "null",
		Version:   "1",
	}, {
		Type:      "out",
		MsgType:   "evt.playback.report",
		ValueType: "string",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.mode.set",
		ValueType: "str_map",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.mode.get_report",
		ValueType: "null",
		Version:   "1",
	}, {
		Type:      "out",
		MsgType:   "evt.mode.report",
		ValueType: "str_map",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.volume.set",
		ValueType: "int",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.volume.get_report",
		ValueType: "null",
		Version:   "1",
	}, {
		Type:      "out",
		MsgType:   "evt.volume.report",
		ValueType: "int",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.mute.set",
		ValueType: "bool",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.mute.get_report",
		ValueType: "null",
		Version:   "1",
	}, {
		Type:      "out",
		MsgType:   "evt.mute.report",
		ValueType: "bool",
		Version:   "1",
	}, {
		Type:      "in",
		MsgType:   "cmd.metadata.get_report",
		ValueType: "null",
		Version:   "1",
	}, {
		Type:      "out",
		MsgType:   "evt.metadata.report",
		ValueType: "str_map",
		Version:   "1",
	}}

	mediaPlayerService := fimptype.Service{
		Name:    "media_player",
		Alias:   "media_player",
		Address: "/rt:dev/rn:bose/ad:1/sv:media_player/ad:",
		Enabled: true,
		Groups:  []string{"ch_0"},
		Props: map[string]interface{}{
			"sup_playback": []string{"play", "pause", "toggle_play_pause", "next_track", "previous_track"},
			"sup_modes":    []string{"repeat", "repeat_one", "shuffle"},
			"sup_metadata": []string{"album", "track", "artist", "image_url"},
		},
		Interfaces: mediaPlayerInterfaces,
	}

	playerID := Player.DeviceID
	manufacturer = "bose"
	name = Player.Name
	serviceAddress := fmt.Sprintf("%s", playerID)
	mediaPlayerService.Address = mediaPlayerService.Address + serviceAddress
	services = append(services, mediaPlayerService)
	deviceAddr = fmt.Sprintf("%s", playerID)
	powerSource := "ac"

	inclReport := fimptype.ThingInclusionReport{
		IntegrationId:     "",
		Address:           deviceAddr,
		Type:              "",
		ProductHash:       manufacturer,
		CommTechnology:    "wifi",
		ProductName:       name,
		ManufacturerId:    manufacturer,
		DeviceId:          playerID,
		HwVersion:         "1",
		SwVersion:         "1",
		PowerSource:       powerSource,
		WakeUpInterval:    "-1",
		Security:          "",
		Tags:              nil,
		Groups:            []string{"ch_0"},
		PropSets:          nil,
		TechSpecificProps: nil,
		Services:          services,
	}

	return inclReport
}
