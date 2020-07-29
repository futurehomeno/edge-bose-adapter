package router

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

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
				msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeStrMap, val, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			}

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
				time.Sleep(time.Second * 1)
				fc.states.Volume, err = fc.client.GetVolume(deviceIP, "8090")
				if err != nil {
					log.Error(err)
				}
				adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
				msg := fimpgo.NewMessage("evt.volume.report", "media_player", fimpgo.VTypeStrMap, fc.states.Volume.Targetvolume, nil, nil, newMsg.Payload)
				fc.mqt.Publish(adr, msg)
			}

		// case "cmd.mode.set":
		// 	deviceIP, err := handler.GetIPFromID(addr, fc.states.Player)
		// 	if err != nil {
		// 		log.Error(err)
		// 	}

		// 	// get str_map including bool values of repeat, repeatOne, crossfade and shuffle
		// 	val, err := newMsg.Payload.GetStrMapValue()
		// 	if err != nil {
		// 		log.Error("Set mode error")
		// 	}

		// 	success, err := fc.pb.ModeSet(val, deviceIP, "8090")
		// 	if err != nil {
		// 		log.Error(err)
		// 	}
		// 	pbStatus, err := fc.client.Get

		case "cmd.standby.set":
			log.Debug("am i here or what")
			fc.pb.PlaybackSet("POWER", "192.168.100.30", "8090")

			// case "cmd.playback.get_report":
			// 	// find groupId from addr(playerId)
			// 	CorrID, err := fc.id.FindGroupFromPlayer(addr, fc.states.Groups)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}

			// 	// send playback status

			// 	success, err := fc.client.GetPlaybackStatus(CorrID, fc.configs.AccessToken)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}
			// 	if success != nil {
			// 		adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
			// 		msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeStrMap, fc.states.PlaybackState, nil, nil, newMsg.Payload)
			// 		fc.mqt.Publish(adr, msg)
			// 	}

			// case "cmd.mode.set":
			// 	// find groupId from addr(playerId)
			// 	CorrID, err := fc.id.FindGroupFromPlayer(addr, fc.states.Groups)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}

			// 	// get str_map including bool values of repeat, repeatOne, crossfade and shuffle
			// 	val, err := newMsg.Payload.GetStrMapValue()
			// 	if err != nil {
			// 		log.Error("Set mode error")
			// 	}

			// 	success, err := fc.pb.ModeSet(val, CorrID, fc.configs.AccessToken)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}
			// 	pbStatus, err := fc.client.GetPlaybackStatus(CorrID, fc.configs.AccessToken)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}
			// 	if success {
			// 		adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
			// 		msg := fimpgo.NewMessage("evt.mode.report", "media_player", fimpgo.VTypeStrMap, pbStatus.PlayModes, nil, nil, newMsg.Payload)
			// 		fc.mqt.Publish(adr, msg)
			// 	}

			// case "cmd.volume.set":
			// 	// find groupId from addr(playerId)
			// 	CorrID, err := fc.id.FindGroupFromPlayer(addr, fc.states.Groups)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}

			// 	// get int from 0-100 representing new volume in %
			// 	val, err := newMsg.Payload.GetIntValue()
			// 	if err != nil {
			// 		log.Error("Volume error", err)
			// 	}

			// 	success, err := fc.vol.VolumeSet(val, CorrID, fc.configs.AccessToken)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}
			// 	currVolume, err := fc.client.GetVolume(CorrID, fc.configs.AccessToken)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}
			// 	if success {
			// 		adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
			// 		msg := fimpgo.NewMessage("evt.volume.report", "media_player", fimpgo.VTypeInt, currVolume.Volume, nil, nil, newMsg.Payload)
			// 		fc.mqt.Publish(adr, msg)
			// 	}

			// case "cmd.mute.set":
			// 	// find groupId fom addr(playerId)
			// 	CorrID, err := fc.id.FindGroupFromPlayer(addr, fc.states.Groups)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}

			// 	// get bool value
			// 	val, err := newMsg.Payload.GetBoolValue()
			// 	if err != nil {
			// 		log.Error("Volume error", err)
			// 	}

			// 	success, err := fc.mute.MuteSet(val, CorrID, fc.configs.AccessToken)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}
			// 	currVolume, err := fc.client.GetVolume(CorrID, fc.configs.AccessToken)
			// 	if err != nil {
			// 		log.Error(err)
			// 	}
			// 	if success {
			// 		adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: addr}
			// 		msg := fimpgo.NewMessage("evt.mute.report", "media_player", fimpgo.VTypeStrMap, currVolume.Muted, nil, nil, newMsg.Payload)
			// 		fc.mqt.Publish(adr, msg)
			// 	}
		}

	case model.ServiceName:
		adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: model.ServiceName, ResourceAddress: "1"}
		switch newMsg.Payload.Type {
		case "cmd.auth.login":
			authReq := model.Login{}
			err := newMsg.Payload.GetObjectValue(&authReq)
			if err != nil {
				log.Error("Incorrect login message ")
				return
			}
			status := model.AuthStatus{
				Status:    model.AuthStateAuthenticated,
				ErrorText: "",
				ErrorCode: "",
			}
			if authReq.Username != "" && authReq.Password != "" {
				// TODO: This is an example . Add your logic here or remove
			} else {
				status.Status = "ERROR"
				status.ErrorText = "Empty username or password"
			}
			fc.appLifecycle.SetAuthState(model.AuthStateAuthenticated)
			msg := fimpgo.NewMessage("evt.auth.status_report", model.ServiceName, fimpgo.VTypeObject, status, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

		case "cmd.auth.set_tokens":
			authReq := model.SetTokens{}
			err := newMsg.Payload.GetObjectValue(&authReq)
			if err != nil {
				log.Error("Incorrect login message ")
				return
			}
			status := model.AuthStatus{
				Status:    model.AuthStateAuthenticated,
				ErrorText: "",
				ErrorCode: "",
			}
			if authReq.AccessToken != "" && authReq.RefreshToken != "" {
				// TODO: This is an example . Add your logic here or remove
			} else {
				status.Status = "ERROR"
				status.ErrorText = "Empty username or password"
			}
			fc.appLifecycle.SetAuthState(model.AuthStateAuthenticated)
			msg := fimpgo.NewMessage("evt.auth.status_report", model.ServiceName, fimpgo.VTypeObject, status, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

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
			msg := fimpgo.NewMessage("evt.app.manifest_report", model.ServiceName, fimpgo.VTypeObject, manifest, nil, nil, newMsg.Payload)
			if err := fc.mqt.RespondToRequest(newMsg.Payload, msg); err != nil {
				// if response topic is not set , sending back to default application event topic
				fc.mqt.Publish(adr, msg)
			}

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
			fc.configs.Param1 = conf.Param1
			fc.configs.Param2 = conf.Param2
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
