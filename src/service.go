package main

import (
	"context"
	"flag"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/thingsplex/bose/bose-api"

	"github.com/futurehomeno/fimpgo"
	"github.com/futurehomeno/fimpgo/discovery"
	"github.com/futurehomeno/fimpgo/edgeapp"
	"github.com/grandcat/zeroconf"
	log "github.com/sirupsen/logrus"
	"github.com/thingsplex/bose/model"
	"github.com/thingsplex/bose/router"
)

func main() {
	var workDir string
	flag.StringVar(&workDir, "c", "", "Work dir")
	flag.Parse()
	if workDir == "" {
		workDir = "./"
	} else {
		fmt.Println("Work dir ", workDir)
	}
	appLifecycle := edgeapp.NewAppLifecycle()
	configs := model.NewConfigs(workDir)
	states := model.NewStates(workDir)
	err := configs.LoadFromFile()
	if err != nil {
		fmt.Print(err)
		panic("Can't load config file.")
	}
	err = states.LoadFromFile()
	if err != nil {
		fmt.Print(err)
		panic("Can't load state file.")
	}
	client := bose.Client{}

	edgeapp.SetupLog(configs.LogFile, configs.LogLevel, configs.LogFormat)
	log.Info("--------------Starting bose----------------")
	log.Info("Work directory : ", configs.WorkDir)

	mqtt := fimpgo.NewMqttTransport(configs.MqttServerURI, configs.MqttClientIdPrefix, configs.MqttUsername, configs.MqttPassword, true, 1, 1)
	err = mqtt.Start()
	responder := discovery.NewServiceDiscoveryResponder(mqtt)
	responder.RegisterResource(model.GetDiscoveryResource())
	responder.Start()

	fimpRouter := router.NewFromFimpRouter(mqtt, appLifecycle, configs, states)
	fimpRouter.Start()

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		states.Player = nil
		for entry := range results {
			log.Println(entry)
			var ip string
			for i := 0; i < len(entry.AddrIPv4); i++ {
				ip = entry.AddrIPv4[i].String()
			}

			players, err := client.GetInfo(ip, entry.Port)
			if err != nil {
				log.Error(err)
			}
			for i := 0; i < len(players); i++ {
				states.Player = append(states.Player, players[i])
			}
			states.SaveToFile()

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

	appLifecycle.SetConnectionState(edgeapp.ConnStateDisconnected)
	appLifecycle.SetAppState(edgeapp.AppStateRunning, nil)
	if states.IsConfigured() && err == nil {
		appLifecycle.SetConnectionState(edgeapp.ConnStateConnected)
	} else {
		appLifecycle.SetConnectionState(edgeapp.ConnStateDisconnected)
	}

	for {
		appLifecycle.WaitForState("main", edgeapp.AppStateRunning)
		ticker := time.NewTicker(time.Duration(15) * time.Second)
		var oldReport map[string]interface{}
		var oldPbStateValue string
		var oldPlayModes map[string]bool
		var oldVolume string
		var oldMuted string
		for ; true; <-ticker.C {
			for i := 0; i < len(states.Player); i++ {
				PlayerIP := states.Player[i].NetworkInfo[0].IpAddress
				states.NowPlaying, err = client.GetNowPlaying(PlayerIP, "8090")
				if err == nil {
					report := map[string]interface{}{
						"album":  states.NowPlaying.Album,
						"track":  states.NowPlaying.Track,
						"artist": states.NowPlaying.Artist,
					}
					adr := &fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeDevice, ResourceName: model.ServiceName, ResourceAddress: "1", ServiceName: "media_player", ServiceAddress: states.Player[i].DeviceID}
					oldReportEqualsNewReport := reflect.DeepEqual(oldReport, report)
					if !oldReportEqualsNewReport {
						msg := fimpgo.NewMessage("evt.metadata.report", "media_player", fimpgo.VTypeStrMap, report, nil, nil, nil)
						mqtt.Publish(adr, msg)
						oldReport = report
						log.Info("New metadata message sent to fimp")
					}

					if oldPbStateValue != states.NowPlaying.PlayStatus && oldPbStateValue != states.NowPlaying.Source {
						if states.NowPlaying.Source == "STANDBY" {
							msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeString, "stop", nil, nil, nil)
							mqtt.Publish(adr, msg)
							oldPbStateValue = states.NowPlaying.Source
						} else if states.NowPlaying.PlayStatus == "PLAY_STATE" {
							msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeString, "play", nil, nil, nil)
							mqtt.Publish(adr, msg)
							oldPbStateValue = states.NowPlaying.PlayStatus
						} else if states.NowPlaying.PlayStatus == "PAUSE_STATE" {
							msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeString, "pause", nil, nil, nil)
							mqtt.Publish(adr, msg)
							oldPbStateValue = states.NowPlaying.PlayStatus
						} else {
							msg := fimpgo.NewMessage("evt.playback.report", "media_player", fimpgo.VTypeString, "unknown", nil, nil, nil)
							mqtt.Publish(adr, msg)
							oldPbStateValue = states.NowPlaying.PlayStatus
						}
						log.Info("New playback.report sent to fimp")
					}
					var shuffle bool
					var repeatOne bool
					var repeat bool
					if states.NowPlaying.ShuffleSetting == "SHUFFLE_OFF" {
						shuffle = false
					} else if states.NowPlaying.ShuffleSetting == "SHUFFLE_ON" {
						shuffle = true
					}
					if states.NowPlaying.RepeatSetting == "REPEAT_OFF" {
						repeatOne = false
						repeat = false
					} else if states.NowPlaying.RepeatSetting == "REPEAT_ONE" {
						repeatOne = true
						repeat = false
					} else if states.NowPlaying.RepeatSetting == "REPEAT_ALL" {
						repeatOne = false
						repeat = true
					}
					newPlayModes := map[string]bool{
						"shuffle":    shuffle,
						"repeat_one": repeatOne,
						"repeat":     repeat,
					}
					oldPlayModesEqualsNewPlayModes := reflect.DeepEqual(oldPlayModes, newPlayModes)
					if !oldPlayModesEqualsNewPlayModes {
						msg := fimpgo.NewMessage("evt.playbackmode.report", "media_player", fimpgo.VTypeBoolMap, newPlayModes, nil, nil, nil)
						mqtt.Publish(adr, msg)
						oldPlayModes = newPlayModes
						log.Info("New playbackmode.report sent to fimp")
					}

					states.Volume, err = client.GetVolume(PlayerIP, "8090")
					if err != nil {
						log.Error(err)
					}
					if oldVolume != states.Volume.Targetvolume {
						vol, err := strconv.Atoi(states.Volume.Targetvolume)
						if err != nil {
							log.Error(err)
						}
						msg := fimpgo.NewMessage("evt.volume.report", "media_player", fimpgo.VTypeInt, vol, nil, nil, nil)
						mqtt.Publish(adr, msg)

						oldVolume = states.Volume.Targetvolume
						log.Info("New volume.report sent to fimp")
					}
					if oldMuted != states.Volume.Muteenabled {
						var mute bool
						if states.Volume.Muteenabled == "true" {
							mute = true
						} else {
							mute = false
						}
						msg := fimpgo.NewMessage("evt.mute.report", "media_player", fimpgo.VTypeBool, mute, nil, nil, nil)
						mqtt.Publish(adr, msg)

						oldMuted = states.Volume.Muteenabled
						log.Info("New mute.report sent to fimp")
					}
				}
			}
		}
		appLifecycle.WaitForState(edgeapp.AppStateNotConfigured, "main")
	}

	mqtt.Stop()
	time.Sleep(5 * time.Second)
}
