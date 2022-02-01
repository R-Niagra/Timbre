package main

import (
	"os"

	"gopkg.in/yaml.v2"
)

var (
	Config Configuration
)

func init() {
	Config = NewConfig("config.yml") // configuration initialized here
}

type Configuration struct {
	D int `yaml:"D"`
}

func NewConfig(fname string) *Configuration {

	f, err := os.Open(fname)
	if err != nil {
		processError(err)
	}

	var cfg Configuration
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		processError(err)
	}

	return cfg
}
