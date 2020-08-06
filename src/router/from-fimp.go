package router

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/thingsplex/bose/bose-api"

	"github.com/thingsplex/bose/handler"

	"github.com/futurehomeno/fimpgo"
	"github.com/futurehomeno/fimpgo/edgeapp"
	log "github.com/sirupsen/logrus"
	"github.com/thingsplex/bose/model"
)

type FromFimpRouter struct {
	inboundMsgCh fimpgo.MessageCh
	mqt          *fimpgo.MqttTransport
	instanceId   string
	appLifecycle *edgeapp.Lifecycle
	configs      *model.Configs
	states       *model.States
	client       *bose.Client
	pb           *handler.Playback
	vol          *handler.Volume
	mute         *handler.Mute
}

func NewFromFimpRouter(mqt *fimpgo.MqttTransport, appLifecycle *edgeapp.Lifecycle, configs *model.Configs, states *model.States) *FromFimpRouter {
	fc := FromFimpRouter{inboundMsgCh: make(fimpgo.MessageCh, 5), mqt: mqt, appLifecycle: appLifecycle, configs: configs, states: states}
	fc.mqt.RegisterChannel("ch1", fc.inboundMsgCh)
	return &fc
}

func (fc *FromFimpRouter) Start() {

	// TODO: Choose either adapter or app topic

	// ------ Adapter topics ---------------------------------------------
	fc.mqt.Subscribe(fmt.Sprintf("pt:j1/+/rt:dev/rn:%s/ad:1/#", model.ServiceName))
	fc.mqt.Subscribe(fmt.Sprintf("pt:j1/+/rt:ad/rn:%s/ad:1", model.ServiceName))

	// ------ Application topic -------------------------------------------
	//fc.mqt.Subscribe(fmt.Sprintf("pt:j1/+/rt:app/rn:%s/ad:1",model.ServiceName))

	go func(msgChan fimpgo.MessageCh) {
		for {
			select {
			case newMsg := <-msgChan:
				fc.routeFimpMessage(newMsg)
			}
		}
	}(fc.inboundMsgCh)
}

func (fc *FromFimpRouter) routeFimpMessage(newMsg *fimpgo.Message) {
	log.Debug("New fimp msg")
	addr := strings.Replace(newMsg.Addr.ServiceAddress, "_0", "", 1)
	switch newMsg.Payload.Service {
	case "media_player":

		switch newMsg.Payload.Type {
		case "cmd.playback.set":
			log.Debug(addr)
			// get "play", "pause", "toggle_play_pause", "next_track" or "previous_track"
			val, err := newMsg.Payload.GetStringValue()
			if err != nil {
				log.Error("Ctrl error")
			}
			deviceIP, err := handler.GetIPFromID(addr, fc.states.Player)
			if err != nil {
				log.Error(err)
			}
			fc.states.NowPlaying, err = fc.client.GetNowPlaying(deviceIP, "8090")
			if err != nil {
				log.Error(err)
			}
			if fc.states.NowPlaying.Source == "STANDBY" { // wake up device
				_, err := fc.pb.WakeUp(deviceIP, "8090")
				if err != nil {
					log.Error(err)
				}
				for i := 0; i < 10; i++ {
					fc.states.NowPlaying, err = fc.client.GetNowPlaying(deviceIP, "8090")
					if err == nil && fc.states.NowPlaying.Source != "STANDBY" && fc.states.NowPlaying.Source != "LOCAL" {
						break
					}
					log.Info("Trying to wake device...")
					time.Sleep(time.Second * 1)
					if i == 10 {
						log.Error("Could not wake device. Please try again.")
					}
				}
			}

			success, err := fc.pb.PlaybackSet(val, deviceIP, "8090")
			if err != nil {
				log.Error(err)
			}
			if success {
				time.Sleep(time.Second * 1)
				fc.states.NowPlaying, err = fc.client.GetNowPlaying(deviceIP, "8090")
				if err != nil {
					log.Error(err)
				}
				if fc.states.NowPlaying.PlayStatus == "STOP_STATE" || fc.states.NowPlaying.PlayStatus == "INVALID_PLAY_STATUS" {
					for i := 0; i < 10; i++ {
						_, err := fc.pb.PlaybackSet(val, deviceIP, "8090")
						if err != nil {
							log.Error(err)
						}
						fc.states.NowPlaying, err = fc.client.GetNowPlaying(deviceIP, "8090")
						if err != nil {
							log.Error(err)
						}
						if fc.states.NowPlaying.PlayStatus != "STOP_STATE" && fc.states.NowPlaying.PlayStatus != "INVALID_PLAY_STATUS" {
							break
						}
						log.Info("Trying to execute command...")
						time.Sleep(time.Second * 1)
						if i == 10 {
							log.Error("Could not execute command. Please try again.")
						}
					}
					_, err := fc.pb.PlaybackSet(val, deviceIP, "8090")
					if err != nil {
						log.Error(err)
					}
					time.Sleep(time.Second * 1)
					fc.states.NowPlaying, err = fc.client.GetNowPlaying(deviceIP, "8090")
					if err != nil {
						log.Error(err)
					}
				}
				var val string
				if fc.states.NowPlaying.PlayStatus == "PLAY_STATE" {
					val = "play"
				} else if fc.states.NowPlaying.PlayStatus == "PAUSE_STATE" {
					val = "pause"
				} else {
					val = "unknown"
				}
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeString, val, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.playback.get_report":
			deviceIP, err := handler.GetIPFromID(addr, fc.states.Player)
			if err != nil {
				log.Error(err)
			}
			fc.states.NowPlaying, err = fc.client.GetNowPlaying(deviceIP, "8090")
			if err != nil {
				log.Error(err)
			}
			if fc.states.NowPlaying.Source == "STANDBY" {
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeStrMap, "stop", nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			} else if fc.states.NowPlaying.Source == "PLAYING" {
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeStrMap, "play", nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			} else if fc.states.NowPlaying.Source == "PAUSED" {
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeStrMap, "pause", nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			} else {
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeStrMap, "unknown", nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.playbackmode.set":
			val, err := newMsg.Payload.GetBoolMapValue()
			if err != nil {
				log.Error("playbackmode error")
			}
			deviceIP, err := handler.GetIPFromID(addr, fc.states.Player)
			if err != nil {
				log.Error(err)
			}
			fc.states.NowPlaying, err = fc.client.GetNowPlaying(deviceIP, "8090")
			if err != nil {
				log.Error(err)
			}
			if fc.states.NowPlaying.Source == "STANDBY" {
				_, err := fc.pb.WakeUp(deviceIP, "8090")
				if err != nil {
					log.Error(err)
				}
				for i := 0; i < 10; i++ {
					fc.states.NowPlaying, err = fc.client.GetNowPlaying(deviceIP, "8090")
					if err == nil && fc.states.NowPlaying.Source != "STANDBY" && fc.states.NowPlaying.Source != "LOCAL" {
						break
					}
					log.Info("Trying to wake device...")
					time.Sleep(time.Second * 1)
					if i == 10 {
						log.Error("Could not wake device. Pleqase try again.")
					}
				}
			}

			success, err := fc.pb.ModeSet(val, deviceIP, "8090")
			if err != nil {
				log.Error(err)
			}
			if success {

				time.Sleep(time.Second * 1)
				fc.states.NowPlaying, err = fc.client.GetNowPlaying(deviceIP, "8090")
				if err != nil {
					log.Error(err)
				}
				var shuffle bool
				var repeatOne bool
				var repeat bool
				if fc.states.NowPlaying.ShuffleSetting == "SHUFFLE_OFF" {
					shuffle = false
				} else if fc.states.NowPlaying.ShuffleSetting == "SHUFFLE_ON" {
					shuffle = true
				}
				if fc.states.NowPlaying.RepeatSetting == "REPEAT_OFF" {
					repeatOne = false
					repeat = false
				} else if fc.states.NowPlaying.RepeatSetting == "REPEAT_ONE" {
					repeatOne = true
					repeat = false
				} else if fc.states.NowPlaying.RepeatSetting == "REPEAT_ALL" {
					repeatOne = false
					repeat = true
				}
				val := map[string]bool{
					"shuffle":    shuffle,
					"repeat_one": repeatOne,
					"repeat":     repeat,
				}
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.playbackmode.report", "media_player", fimpgo.VTypeBoolMap, val, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.playbackmode.get_report":
			deviceIP, err := handler.GetIPFromID(addr, fc.states.Player)
			if err != nil {
				log.Error(err)
			}
			fc.states.NowPlaying, err = fc.client.GetNowPlaying(deviceIP, "8090")
			if err != nil {
				log.Error(err)
			}
			var shuffle bool
			var repeatOne bool
			var repeat bool
			if fc.states.NowPlaying.ShuffleSetting == "SHUFFLE_OFF" {
				shuffle = false
			} else if fc.states.NowPlaying.ShuffleSetting == "SHUFFLE_ON" {
				shuffle = true
			}
			if fc.states.NowPlaying.RepeatSetting == "REPEAT_OFF" {
				repeatOne = false
				repeat = false
			} else if fc.states.NowPlaying.RepeatSetting == "REPEAT_ONE" {
				repeatOne = true
				repeat = false
			} else if fc.states.NowPlaying.RepeatSetting == "REPEAT_ALL" {
				repeatOne = false
				repeat = true
			}
			val := map[string]bool{
				"shuffle":    shuffle,
				"repeat_one": repeatOne,
				"repeat":     repeat,
			}
			adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
			msg := fimpgo.NewMessage("evt.playbackmode.report", "media_player", fimpgo.VTypeBoolMap, val, nil, nil, newMsg.Payload)
			fc.mqt.Publish(adr, msg)

		case "cmd.volume.set":
			deviceIP, err := handler.GetIPFromID(addr, fc.states.Player)
			if err != nil {
				log.Error(err)
			}

			// get int from 0-100 representing new volume in %
			val, err := newMsg.Payload.GetIntValue()
			if err != nil {
				log.Error("Volume error", err)
			}

			success, err := fc.vol.VolumeSet(val, deviceIP, "8090")
			if err != nil {
				log.Error(err)
			}
			if success {
				time.Sleep(time.Second * (1 / 2))
				fc.states.Volume, err = fc.client.GetVolume(deviceIP, "8090")
				if err != nil {
					log.Error(err)
				}
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.volume.report", "media_player", fimpgo.VTypeStrMap, fc.states.Volume.Targetvolume, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.volume.get_report":
			deviceIP, err := handler.GetIPFromID(addr, fc.states.Player)
			if err != nil {
				log.Error(err)
			}
			fc.states.Volume, err = fc.client.GetVolume(deviceIP, "8090")
			if err != nil {
				log.Error(err)
			}
			adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
			msg := fimpgo.NewMessage("evt.volume.report", "media_player", fimpgo.VTypeStrMap, fc.states.Volume.Targetvolume, nil, nil, newMsg.Payload)
			fc.mqt.Publish(adr, msg)

		case "cmd.mute.set":
			deviceIP, err := handler.GetIPFromID(addr, fc.states.Player)
			if err != nil {
				log.Error(err)
			}

			success, err := fc.mute.MuteSet(deviceIP, "8090")
			if err != nil {
				log.Error(err)
			}
			if success {
				time.Sleep(time.Second * (1 / 2))
				fc.states.Volume, err = fc.client.GetVolume(deviceIP, "8090")
				if err != nil {
					log.Error(err)
				}
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.volume.report", "media_player", fimpgo.VTypeStrMap, fc.states.Volume.Muteenabled, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.mute.get_report":
			deviceIP, err := handler.GetIPFromID(addr, fc.states.Player)
			if err != nil {
				log.Error(err)
			}
			fc.states.Volume, err = fc.client.GetVolume(deviceIP, "8090")
			if err != nil {
				log.Error(err)
			}
			adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
			msg := fimpgo.NewMessage("evt.volume.report", "media_player", fimpgo.VTypeStrMap, fc.states.Volume.Muteenabled, nil, nil, newMsg.Payload)
			fc.mqt.Publish(adr, msg)

		case "cmd.standby.set":
			// this is only for testing
			fc.pb.PlaybackSet("POWER", "192.168.100.30", "8090")
		}

	case model.ServiceName:
		adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: model.ServiceName, ResourceAddress: "1"}
		switch newMsg.Payload.Type {

		case "cmd.app.get_manifest":
			mode, err := newMsg.Payload.GetStringValue()
			if err != nil {
				log.Error("Incorrect request format ")
				return
			}
			manifest := edgeapp.NewManifest()
			err = manifest.LoadFromFile(filepath.Join(fc.configs.GetDefaultDir(), "app-manifest.json"))
			if err != nil {
				log.Error("Failed to load manifest file .Error :", err.Error())
				return
			}
			if mode == "manifest_state" {
				manifest.AppState = *fc.appLifecycle.GetAllStates()
				manifest.ConfigState = fc.configs
			}
			if fc.states.IsConfigured() {
				var playerSelect []interface{}
				manifest.Configs[0].ValT = "str_map"
				manifest.Configs[0].UI.Type = "list_checkbox"
				for i := 0; i < len(fc.states.Player); i++ {
					label := fc.states.Player[i].Name
					playerSelect = append(playerSelect, map[string]interface{}{"val": fc.states.Player[i].DeviceID, "label": map[string]interface{}{"en": label}})
				}
				manifest.Configs[0].UI.Select = playerSelect
			} else {
				manifest.Configs[0].ValT = "string"
				manifest.Configs[0].UI.Type = "input_readonly"
				var val edgeapp.Value
				val.Default = "Found no players on your network. Make sure that the smarthub and the player is on the same network and scan again."
				manifest.Configs[0].Val = val
			}

			msg := fimpgo.NewMessage("evt.app.manifest_report", model.ServiceName, fimpgo.VTypeObject, manifest, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.system.sync":
			// // scan network again
			resolver, err := zeroconf.NewResolver(nil)
			if err != nil {
				log.Fatalln("Failed to initialize resolver:", err.Error())
			}

			entries := make(chan *zeroconf.ServiceEntry)
			go func(results <-chan *zeroconf.ServiceEntry) {
				fc.states.Player = nil
				for entry := range results {
					log.Println(entry)
					var ip string
					for i := 0; i < len(entry.AddrIPv4); i++ {
						ip = entry.AddrIPv4[i].String()
					}

					players, err := fc.client.GetInfo(ip, entry.Port)
					if err != nil {
						log.Error(err)
					}
					for i := 0; i < len(players); i++ {
						fc.states.Player = append(fc.states.Player, players[i])
					}
					fc.states.SaveToFile()

				}
				log.Println("No more entries.")
			}(entries)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			err = resolver.Browse(ctx, "._soundtouch._tcp", ".local", entries)
			if err != nil {
				log.Fatalln("Failed to browse:", err.Error())
			}

			<-ctx.Done()

		case "cmd.app.get_state":
			msg := fimpgo.NewMessage("evt.app.manifest_report", model.ServiceName, fimpgo.VTypeObject, fc.appLifecycle.GetAllStates(), nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.config.get_extended_report":

			msg := fimpgo.NewMessage("evt.config.extended_report", model.ServiceName, fimpgo.VTypeObject, fc.configs, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.config.extended_set":
			conf := model.Configs{}
			err := newMsg.Payload.GetObjectValue(&conf)
			if err != nil {
				// TODO: This is an example . Add your logic here or remove
				log.Error("Can't parse configuration object")
				return
			}
			fc.configs.WantedPlayers = conf.WantedPlayers
			fc.configs.SaveToFile()
			log.Debugf("App reconfigured . New parameters : %v", fc.configs)
			// TODO: This is an example . Add your logic here or remove
			configReport := edgeapp.ConfigReport{
				OpStatus: "ok",
				AppState: *fc.appLifecycle.GetAllStates(),
			}
			msg := fimpgo.NewMessage("evt.app.config_report", model.ServiceName, fimpgo.VTypeObject, configReport, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				fc.mqt.Publish(adr, msg)
			}

			log.Debug(fc.configs.WantedPlayers)
			for i := 0; i < len(fc.configs.WantedPlayers); i++ {
				PlayerID := fc.configs.WantedPlayers[i]
				log.Debug("wanted PlayerID: ", PlayerID)
				for p := 0; p < len(fc.states.Player); p++ {
					log.Debug("actual playerID: ", fc.states.Player[p].DeviceID)
					if PlayerID == fc.states.Player[p].DeviceID {
						inclReport := model.MakeInclusionReport(fc.states.Player[p])

						msg := fimpgo.NewMessage("evt.thing.inclusion_report", "bose", fimpgo.VTypeObject, inclReport, nil, nil, nil)
						adr := fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: "bose", ResourceAddress: "1"}
						fc.mqt.Publish(&adr, msg)
					}
				}
				fc.appLifecycle.SetConfigState(model.ConfigStateConfigured)
			}

		case "cmd.log.set_level":
			// Configure log level
			level, err := newMsg.Payload.GetStringValue()
			if err != nil {
				return
			}
			logLevel, err := log.ParseLevel(level)
			if err == nil {
				log.SetLevel(logLevel)
				fc.configs.LogLevel = level
				fc.configs.SaveToFile()
			}
			log.Info("Log level updated to = ", logLevel)

		case "cmd.system.reconnect":
			// This is optional operation.
			//val := map[string]string{"status":status,"error":errStr}
			val := model.ButtonActionResponse{
				Operation:       "cmd.system.reconnect",
				OperationStatus: "ok",
				Next:            "config",
				ErrorCode:       "",
				ErrorText:       "",
			}
			msg := fimpgo.NewMessage("evt.app.config_action_report", model.ServiceName, fimpgo.VTypeObject, val, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.app.factory_reset":
			val := model.ButtonActionResponse{
				Operation:       "cmd.app.factory_reset",
				OperationStatus: "ok",
				Next:            "config",
				ErrorCode:       "",
				ErrorText:       "",
			}
			fc.appLifecycle.SetConfigState(model.ConfigStateNotConfigured)
			fc.appLifecycle.SetAppState(model.AppStateNotConfigured, nil)
			fc.appLifecycle.SetAuthState(model.AuthStateNotAuthenticated)
			msg := fimpgo.NewMessage("evt.app.config_action_report", model.ServiceName, fimpgo.VTypeObject, val, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.network.get_all_nodes":
			// TODO: This is an example . Add your logic here or remove
		case "cmd.thing.get_inclusion_report":
			//nodeId , _ := newMsg.Payload.GetStringValue()
			// TODO: This is an example . Add your logic here or remove
		case "cmd.thing.inclusion":
			//flag , _ := newMsg.Payload.GetBoolValue()
			// TODO: This is an example . Add your logic here or remove
		case "cmd.thing.delete":
			// remove device from network
			val, err := newMsg.Payload.GetStrMapValue()
			if err != nil {
				log.Error("Wrong msg format")
				return
			}
			deviceId, ok := val["address"]
			if ok {
				// TODO: This is an example . Add your logic here or remove
				log.Info(deviceId)
			} else {
				log.Error("Incorrect address")

			}
		}
		fc.configs.SaveToFile()
		fc.states.SaveToFile()
	}
}
