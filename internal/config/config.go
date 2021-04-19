package config

import (
	"io/ioutil"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/s-newman/image-shepherd/pkg/image"
)

type Config struct {
	Images []image.Image
}

func Load(path string) Config {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		zap.S().Fatalf("Failed to read config file: %s", err)
	}

	var c Config
	err = yaml.Unmarshal(f, &c)
	if err != nil {
		zap.S().Fatalf("Failed to parse YAML: %s", err)
	}

	return c
}
