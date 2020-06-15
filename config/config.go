package config

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

// Config represents a collection of Metrics.
type Config struct {
	Metrics []Metric
}

// Metric represents all the information needed for an SNMP metric.
type Metric struct {
	Name            string `yaml:"name"`
	Description     string `yaml:"description"`
	OidStub         string `yaml:"oidStub"`
	MlabUplinkName  string `yaml:"mlabUplinkName"`
	MlabMachineName string `yaml:"mlabMachineName"`
}

// New returns a new Config struct.
func New(yamlFile string) (Config, error) {
	var c Config

	yamlData, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		log.Printf("ERROR: failed to read YAML metrics config file '%v': %v", yamlFile, err)
		return c, err
	}

	err = yaml.UnmarshalStrict(yamlData, &c.Metrics)
	if err != nil {
		log.Printf("ERROR: failed to unmarshal YAML metrics config: %v", err)
		return c, err
	}

	return c, err
}
