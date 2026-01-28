package main

import (
	"log"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type ConfigItem struct {
	Name       string
	Regex      []string
	SendToMain bool `yaml:"send-to-main"`
}
type Config struct {
	Destinations []ConfigItem
}

type Destination struct {
	url        string
	regexp     []string
	sendToMain bool
}

func (dest Destination) Matches(body string) bool {
	for _, regex := range dest.regexp {
		match, _ := regexp.MatchString(regex, body)
		if match {
			return true
		}
	}
	return false
}

var MainDestination Destination

var LoadedConfig Config
var LoadedDestinations []Destination
var ConfigIsLoaded = false

func loadDestinations() {
	if ConfigIsLoaded {
		return
	}

	// Initialize MainDestination (after .env is loaded)
	MainDestination = Destination{
		url: os.Getenv("WEBHOOK_MAIN_URL"),
	}
	if MainDestination.url == "" {
		log.Fatal("Missing WEBHOOK_MAIN_URL environment variable")
	}

	config, _ := os.ReadFile("config.yml")
	err := yaml.Unmarshal(config, &LoadedConfig)
	if err != nil {
		panic(err)
	}

	// Convert config into destinations
	for _, item := range LoadedConfig.Destinations {
		envVar := "WEBHOOK_" + item.Name + "_URL"
		dest := Destination{
			url:        os.Getenv(envVar),
			regexp:     item.Regex,
			sendToMain: item.SendToMain,
		}
		if dest.url == "" {
			log.Fatalf("Missing %s environment variable", envVar)
		}

		LoadedDestinations = append(LoadedDestinations, dest)
		log.Printf("Loaded destination: %s", item.Name)
	}

	log.Printf("Configuration complete: %d destinations loaded", len(LoadedDestinations))
	ConfigIsLoaded = true
}

func destinations(body string) []Destination {
	var results []Destination
	for _, dest := range LoadedDestinations {
		if dest.Matches(body) {
			results = append(results, dest)
		}
	}
	return results
}
