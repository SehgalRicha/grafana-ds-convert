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
	DirectIRONdb        bool   `json:"direct_irondb" toml:"direct_irondb" yaml:"direct_irondb"`
	APIToken            string `json:"api_token" toml:"api_token" yaml:"api_token"`
	Host                string `json:"host" toml:"host" yaml:"host"`
	Port                string `json:"port" toml:"port" yaml:"port"`
	StatsdFlushInterval int    `json:"statsd_interval" toml:"statsd_interval" yaml:"statsd_interval"`
	StatsdAggregations  `json:"statsd_aggregations" toml:"statsd_aggregations" yaml:"statsd_aggregations"`
}

// Grafana defines the Grafana specific configuration options
type Grafana struct {
	Host                string   `json:"host" toml:"host" yaml:"host"`
	Port                string   `json:"port" toml:"port" yaml:"port"`
	Path                string   `json:"path" toml:"path" yaml:"path"`
	APIToken            string   `json:"api_token" toml:"api_token" yaml:"api_token"`
	AnonymousAuth       bool     `json:"anonymous_auth" toml:"anonymous_auth" yaml:"anonymous_auth"`
	TLS                 bool     `json:"secure" toml:"secure" yaml:"secure"`
	SourceFolder        string   `json:"src_folder" toml:"src_folder" yaml:"src_folder"`
	DestinationFolder   string   `json:"dest_folder" toml:"dest_folder" yaml:"dest_folder"`
	GraphiteDatasources []string `json:"graphite_datasources" toml:"graphite_datasources" yaml:"graphite_datasources"`
	CirconusDatasource  string   `json:"circonus_datasource" toml:"circonus_datasource" yaml:"circonus_datasource"`
	NoAlerts            bool     `json:"no_alerts" toml:"no_alerts" yaml:"no_alerts"`
}

// StatsdAggregations defines the statsd_aggregations options
type StatsdAggregations struct {
	Remove          bool     `json:"remove" toml:"remove" yaml:"remove"`
	AggregationList []string `json:"agg_list" toml:"agg_list" yaml:"agg_list"`
	Period          int      `json:"period" toml:"period" yaml:"period"`
}

// Validate validates that the required config keys are set
func Validate() error {
	if viper.GetString(keys.GrafanaAPIToken) == "" && !viper.GetBool(keys.GrafanaAnonymousAuth) {
		return errors.New("Grafana API Token must be set")
	} else if viper.GetString(keys.GrafanaHost) == "" {
		return errors.New("Grafana host must be set")
	}
	if viper.GetString(keys.GrafanaSourceFolder) == "" || viper.GetString(keys.GrafanaDestFolder) == "" {
		return errors.New("must provide source and destination Grafana folders")
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
