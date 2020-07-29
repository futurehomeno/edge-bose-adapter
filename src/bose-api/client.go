package bose

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type Config struct {
	ErrorCode  int    `xml:"errorCode"`
	Message    string `xml:"message"`
	StatusCode int    `xml:"statusCode"`
	Success    bool   `xml:"success"`
}

type Client struct {
	httResponse *http.Response
	NowPlaying  `xml:"nowPlaying"`
	Info        []Player `xml:"info"`
	Volume      `xml:"volume"`
}

type NowPlaying struct {
	XMLNamePlaying xml.Name `xml:"nowPlaying"`
	Source         string   `xml:"source,attr"`
	SourceAccount  string   `xml:"sourceAccount,attr"`
	ContentItem    struct {
		Text          string `xml:",chardata"`
		Source        string `xml:"source,attr"`
		Type          string `xml:"type,attr"`
		Location      string `xml:"location,attr"`
		SourceAccount string `xml:"sourceAccount,attr"`
		IsPresetable  string `xml:"isPresetable,attr"`
		ItemName      string `xml:"itemName"`
	} `xml:"ContentItem"`
	Track       string `xml:"track"`
	Artist      string `xml:"artist"`
	Album       string `xml:"album"`
	StationName string `xml:"stationName"`
	Art         struct {
		Text           string `xml:",chardata"`
		ArtImageStatus string `xml:"artImageStatus,attr"`
	} `xml:"art"`
	Time struct {
		Text  string `xml:",chardata"`
		Total string `xml:"total,attr"`
	} `xml:"time"`
	SkipEnabled         string `xml:"skipEnabled"`
	PlayStatus          string `xml:"playStatus"`
	ShuffleSetting      string `xml:"shuffleSetting"`
	RepeatSetting       string `xml:"repeatSetting"`
	SkipPreviousEnabled string `xml:"skipPreviousEnabled"`
	SeekSupported       struct {
		Text  string `xml:",chardata"`
		Value string `xml:"value,attr"`
	} `xml:"seekSupported"`
	Account struct {
		Text             string `xml:",chardata"`
		SubscriptionType string `xml:"subscriptionType,attr"`
	} `xml:"account"`
	StreamType string `xml:"streamType"`
	TrackID    string `xml:"trackID"`
}

type Player struct {
	XMLName          xml.Name `xml:"info"`
	Text             string   `xml:",chardata"`
	DeviceID         string   `xml:"deviceID,attr"`
	Name             string   `xml:"name"`
	Type             string   `xml:"type"`
	MargeAccountUUID string   `xml:"margeAccountUUID"`
	Components       struct {
		Text      string `xml:",chardata"`
		Component []struct {
			Text              string `xml:",chardata"`
			ComponentCategory string `xml:"componentCategory"`
			SoftwareVersion   string `xml:"softwareVersion"`
			SerialNumber      string `xml:"serialNumber"`
		} `xml:"component"`
	} `xml:"components"`
	MargeURL    string `xml:"margeURL"`
	NetworkInfo []struct {
		Text       string `xml:",chardata"`
		Type       string `xml:"type,attr"`
		MacAddress string `xml:"macAddress"`
		IpAddress  string `xml:"ipAddress"`
	} `xml:"networkInfo"`
	ModuleType  string `xml:"moduleType"`
	Variant     string `xml:"variant"`
	VariantMode string `xml:"variantMode"`
	CountryCode string `xml:"countryCode"`
	RegionCode  string `xml:"regionCode"`
}

type Volume struct {
	XMLNameVolume  xml.Name `xml:"volume"`
	TextVolume     string   `xml:",chardata"`
	DeviceIDVolume string   `xml:"deviceID,attr"`
	Targetvolume   string   `xml:"targetvolume"`
	Actualvolume   string   `xml:"actualvolume"`
	Muteenabled    string   `xml:"muteenabled"`
}

func (c *Client) GetInfo(ip string, port int) ([]Player, error) {

	url := fmt.Sprintf("%s%s%s%s%s", "http://", ip, ":", strconv.Itoa(port), "/info")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("Error when getting info: ", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml")
	// log.Debug("New Request: ", req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Error when DefaultClient.Do on GetInfo: ", err)
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("Error when ioutil.ReadAll on GetInfo: ", err)
		return nil, err
	}
	err = xml.Unmarshal(body, &c.Info)
	if err != nil {
		log.Error("Error when unmarshaling body: ", err)
		return nil, err
	}
	return c.Info, nil
}

func (c *Client) GetNowPlaying(ip string, port string) (NowPlaying, error) {
	url := fmt.Sprintf("%s%s%s%s%s", "http://", ip, ":", port, "/now_playing")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("Error when getting nowPlaying: ", err)
		return c.NowPlaying, err
	}

	log.Debug("New Request: ", req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Error when DefaultClient.Do on nowPlaying: ", err)
		return c.NowPlaying, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("Error when ioutil.ReadAll on nowPlaying: ", err)
		return c.NowPlaying, err
	}
	err = xml.Unmarshal(body, &c)
	log.Debug("source: ", c.NowPlaying.Source)
	if err != nil {
		log.Error("Error when unmarshaling body: ", err)
		return c.NowPlaying, err
	}
	return c.NowPlaying, nil
}

func (c *Client) GetVolume(ip string, port string) (Volume, error) {
	url := fmt.Sprintf("%s%s%s%s%s", "http://", ip, ":", port, "/volume")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("Error when getting volume: ", err)
		return c.Volume, err
	}

	log.Debug("New Request: ", req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Error when DefaultClient.Do on getVolume: ", err)
		return c.Volume, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("Error when ioutil.ReadAll on getVolume: ", err)
		return c.Volume, err
	}
	err = xml.Unmarshal(body, &c)
	if err != nil {
		log.Error("Error when unmarshaling body: ", err)
		return c.Volume, err
	}
	return c.Volume, nil
}

func processHTTPResponse(resp *http.Response, err error, holder interface{}) error {
	if err != nil {
		log.Error(fmt.Errorf("API does not respond"))
		return err
	}
	defer resp.Body.Close()
	// check http return code
	if resp.StatusCode != 200 {
		//bytes, _ := ioutil.ReadAll(resp.Body)
		log.Error("Bad HTTP return code ", resp.StatusCode)
		return fmt.Errorf("Bad HTTP return code %d", resp.StatusCode)
	}

	// Unmarshall response into given struct
	if err = xml.NewDecoder(resp.Body).Decode(holder); err != nil {
		return err
	}
	return nil
}
