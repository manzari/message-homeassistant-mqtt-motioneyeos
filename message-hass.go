package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	proto "github.com/huin/mqtt"
	"github.com/jeffallen/mqtt"
)

var config MqttConfig
var configPath = "/data/etc/hass.json"

type CameraConfig struct {
	DeviceName        string
	DeiceFriendlyName string
	DeviceClass       string
}

type MqttConfig struct {
	Host       string
	User       string
	Pass       string
	Dump       bool
	Retain     bool
	BaseTopic  string
	AutoConfig bool
	Cameras    []CameraConfig
}

type ConfigMessage struct {
	Name        string `json:"name"`
	DeviceClass string `json:"device_class"`
	StateTopic  string `json:"state_topic"`
}

func main() {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := MqttConfig{
			Host:      "localhost:1883",
			User:      "user",
			Pass:      "70p53cr37",
			Dump:      false,
			Retain:    false,
			BaseTopic: "homeassistant",
			Cameras: []CameraConfig{
				{
					DeviceName:        "entrance_camera_motion",
					DeiceFriendlyName: "Entrance Motion",
					DeviceClass:       "motion",
				},
			},
			AutoConfig: true,
		}
		defaultConfigJson, _ := json.Marshal(defaultConfig)
		err = ioutil.WriteFile(configPath, defaultConfigJson, 0600)
		if err != nil {
			fmt.Fprint(os.Stderr, "Failed to write default config file: ", err)
			os.Exit(1)
		}

	}
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Fprint(os.Stderr, "Failed to read config file: ", err)
		os.Exit(1)
	}
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		fmt.Fprint(os.Stderr, "Failed to parse config file: ", err)
		os.Exit(1)
	}

	index := flag.Int("cam", 0, "Index of the camera in the config.  Defaults to the first camera")

	flag.Parse()
	if flag.Arg(0) != "ON" && flag.Arg(0) != "OFF" {
		fmt.Fprintln(os.Stderr, "usage: message-hass -cam=<INDEX> <ON/OFF>")
		os.Exit(1)
	}

	if *index >= len(config.Cameras) {
		fmt.Fprint(os.Stderr, "Failed to load camera in config file at index: ", *index)
		os.Exit(1)
	}

	conn, err := net.Dial("tcp", config.Host)
	if err != nil {
		fmt.Fprint(os.Stderr, "dial: ", err)
		return
	}
	cc := mqtt.NewClientConn(conn)
	cc.Dump = config.Dump

	if err := cc.Connect(config.User, config.Pass); err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Connected with client id", cc.ClientId)

	camera := config.Cameras[*index]
	stateTopic := config.BaseTopic + "/binary_sensor/" + camera.DeviceName + "/state"
	if config.AutoConfig {
		configTopic := config.BaseTopic + "/binary_sensor/" + camera.DeviceName + "/config"
		configMessage := ConfigMessage{camera.DeiceFriendlyName, camera.DeviceClass, stateTopic}
		jsonConfigMessage, err := json.Marshal(&configMessage)
		if err != nil {
			fmt.Fprint(os.Stderr, "json marshal failed: ", err)
			os.Exit(1)
		}
		cc.Publish(&proto.Publish{
			Header:    proto.Header{Retain: config.Retain},
			TopicName: configTopic,
			Payload:   proto.BytesPayload(jsonConfigMessage),
		})
		fmt.Println("Published autoconfig")
	}
	cc.Publish(&proto.Publish{
		Header:    proto.Header{Retain: config.Retain},
		TopicName: stateTopic,
		Payload:   proto.BytesPayload([]byte(flag.Arg(0))),
	})
	fmt.Println("Published state: ", flag.Arg(0))
	cc.Disconnect()
}
