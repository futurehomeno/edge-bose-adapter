package main

import (
	"context"
	"flag"
	"fmt"
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
		ticker := time.NewTicker(time.Duration(30) * time.Second)
		for ; true; <-ticker.C {
			// pb := handler.Playback{}
			for i := 0; i < len(states.Player); i++ {
				// if appLifecycle.ConfigState() == edgeapp.ConfigStateNotConfigured {
				// 	log.Debug("I SHOULD SEND INCLUSION REPORT")
				// 	inclReport := model.MakeInclusionReport(states.Player[i])

				// 	msg := fimpgo.NewMessage("evt.thing.inclusion_report", "bose", fimpgo.VTypeObject, inclReport, nil, nil, nil)
				// 	adr := fimpgo.Address{MsgType: fimpgo.MsgTypeEvt, ResourceType: fimpgo.ResourceTypeAdapter, ResourceName: "bose", ResourceAddress: "1"}
				// 	mqtt.Publish(&adr, msg)
				// 	appLifecycle.SetConfigState(edgeapp.ConfigStateConfigured)
				// }
				// ip := states.Player[i].NetworkInfo[0].IpAddress
				// nowplaying, err := client.GetNowPlaying(ip, "8090")
				// if err != nil {
				// 	log.Error(err)
				// }

				// log.Debug("Nowplaying.Source: ", nowplaying.NowPlaying.Source)
				// if nowplaying.NowPlaying.Source == "STANDBY" {
				// 	log.Debug("Source is STANDBY")
				// 	pb.PlaybackSet("POWER", ip, "8090")
				// }
				// if nowplaying.NowPlaying.Source != "STANDBY" {
				// 	log.Debug("Playstatus: ", nowplaying.NowPlaying.PlayStatus)
				// }
				// afterplaying, err := client.GetNowPlaying(ip, "8090")
				// if err != nil {
				// 	log.Error(err)
				// }
				// log.Debug("STATE AFTER LOOP: ", afterplaying.NowPlaying.PlayStatus)
				// 	// pb.PlaybackSet("toggle_play_pause", i
			}
		}
		// Configure custom resources here
		//if err := conFimpRouter.Start(); err !=nil {
		//	appLifecycle.PublishEvent(model.EventConfigError,"main",nil)
		//}else {
		//	appLifecycle.WaitForState(model.StateConfiguring,"main")
		//}
		//TODO: Add logic here
		appLifecycle.WaitForState(edgeapp.AppStateNotConfigured, "main")
	}

	mqtt.Stop()
	time.Sleep(5 * time.Second)
}
