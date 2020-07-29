package model

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/thingsplex/bose/bose-api"

	log "github.com/sirupsen/logrus"
	"github.com/thingsplex/bose/utils"
)

type States struct {
	path         string
	LogFile      string          `json:"log_file"`
	LogLevel     string          `json:"log_level"`
	LogFormat    string          `json:"log_format"`
	WorkDir      string          `json:"-"`
	ConfiguredAt string          `json:"configuret_at"`
	ConfiguredBy string          `json:"configured_by"`
	Player       []bose.Player   `xml:"info"`
	NowPlaying   bose.NowPlaying `xml:"nowPlaying"`
	Volume       bose.Volume     `xml:"volume"`
}

func NewStates(workDir string) *States {
	state := &States{WorkDir: workDir}
	state.path = filepath.Join(workDir, "data", "state.xml")
	if !utils.FileExists(state.path) {
		log.Info("State file doesn't exist.Loading default state")
		defaultStateFile := filepath.Join(workDir, "defaults", "state.xml")
		err := utils.CopyFile(defaultStateFile, state.path)
		if err != nil {
			fmt.Print(err)
			panic("Can't copy state file.")
		}
	}
	return state
}

func (st *States) LoadFromFile() error {
	stateFileBody, err := ioutil.ReadFile(st.path)
	if err != nil {
		return err
	}
	err = xml.Unmarshal(stateFileBody, st)
	if err != nil {
		return err
	}
	return nil
}

func (st *States) SaveToFile() error {
	st.ConfiguredBy = "auto"
	st.ConfiguredAt = time.Now().Format(time.RFC3339)
	bpayload, err := xml.Marshal(st)
	err = ioutil.WriteFile(st.path, bpayload, 0664)
	if err != nil {
		return err
	}
	return err
}

func (st *States) GetDataDir() string {
	return filepath.Join(st.WorkDir, "data")
}

func (st *States) GetDefaultDir() string {
	return filepath.Join(st.WorkDir, "defaults")
}

func (st *States) LoadDefaults() error {
	stateFile := filepath.Join(st.WorkDir, "data", "state.xml")
	os.Remove(stateFile)
	log.Info("State file doesn't exist.Loading default state")
	defaultStateFile := filepath.Join(st.WorkDir, "defaults", "state.xml")
	return utils.CopyFile(defaultStateFile, stateFile)
}

func (st *States) IsConfigured() bool {
	if len(st.Player) != 0 {
		return true
	}
	return false
}

type StateReport struct {
	OpStatus string    `json:"op_status"`
	AppState AppStates `json:"app_state"`
}
