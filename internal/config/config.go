package config

import (
	"os"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/HackUCF/image-shepherd/pkg/image"
)

type Config struct {
	Images           []image.Image
	OwnerProjectID   string `yaml:"owner_project_id,omitempty"`
	RequireProtected bool   `yaml:"require_protected,omitempty"`
}

func Load(path string) Config {
	f, err := os.ReadFile(path)
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
