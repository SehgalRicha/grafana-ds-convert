package config

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/circonus/grafana-ds-convert/internal/config/keys"
	toml "github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

// Config defines the running configuration options
type Config struct {
	Circonus Circonus `json:"circonus" toml:"circonus" yaml:"circonus"`
	Grafana  Grafana  `json:"grafana" toml:"grafana" yaml:"grafana"`
	Debug    bool     `json:"debug" toml:"debug" yaml:"debug"`
}

// Circonus defines the Circonus specific configuration options
type Circonus struct {
	IRONdbHost string `json:"irondb_host" toml:"irondb_host" yaml:"irondb_host"`
}

// Grafana defines the Grafana specific configuration options
type Grafana struct {
	Host              string `json:"host" toml:"host" yaml:"host"`
	Port              string `json:"port" toml:"port" yaml:"port"`
	APIToken          string `json:"api_token" toml:"api_token" yaml:"api_token"`
	TLS               bool   `json:"secure" toml:"secure" yaml:"secure"`
	SourceFolder      string `json:"src_folder" toml:"src_folder" yaml:"src_folder"`
	DestinationFolder string `json:"dest_folder" toml:"dest_folder" yaml:"dest_folder"`
}

// Validate validates that the required config keys are set
func Validate() error {
	if viper.GetString(keys.GrafanaAPIToken) == "" {
		return errors.New("Grafana API Token must be set")
	} else if viper.GetString(keys.GrafanaHost) == "" || viper.GetString(keys.GrafanaPort) == "" {
		return errors.New("Grafana host and port must be set")
	}
	return nil
}

// getConfig dumps the current configuration and returns it
func getConfig() (*Config, error) {
	var cfg Config

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, errors.Wrap(err, "parsing config")
	}

	return &cfg, nil
}

// ShowConfig prints the running configuration
func ShowConfig(w io.Writer) error {
	var cfg *Config
	var err error
	var data []byte

	cfg, err = getConfig()
	if err != nil {
		return err
	}

	format := viper.GetString(keys.ShowConfig)

	switch format {
	case "json":
		data, err = json.MarshalIndent(cfg, " ", "  ")
	case "yaml":
		data, err = yaml.Marshal(cfg)
	case "toml":
		data, err = toml.Marshal(*cfg)
	default:
		return errors.Errorf("unknown config format '%s'", format)
	}

	if err != nil {
		return errors.Wrapf(err, "formatting config (%s)", format)
	}

	fmt.Fprintln(w, string(data))
	return nil
}
