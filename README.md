# grafana-ds-convert

[![CodeQL](https://github.com/circonus/grafana-ds-convert/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/circonus/grafana-ds-convert/actions/workflows/codeql-analysis.yml)

grafana-ds-convert allows Grafana users to convert assets like dashboards and alerts from different supported query languages to Circonus Analytics Query Language (CAQL).

## Features

1. TOML, YAML, and JSON config file support
1. Translate Graphite queries for dashboard panels into CAQL queries

## Configuration Options

```sh
Usage:
  grafana-ds-convert [flags]

Flags:
  -c, --config string        config file (default: $HOME/.grafana-ds-convert.yaml|.json|.toml)
  -h, --help                 help for grafana-ds-convert
      --show-config string   show config (json|toml|yaml) and exit
  -v, --version              show version and exit
  ```

## Example TOML Configuration File
```toml
# Global settings
debug = false

# Circonus section defines connection params to either
# IRONdb directly or the Circonus API
[circonus]
  direct_irondb = false # whether or not to communicate directly with IRONdb
  host = "api.circonus.com" # defaults to api.circonus.com, can be set to IRONdb node URI
  port = "" # defaults to empty for Circonus API, set to HTTP port of IRONdb for direct IRONdb functionality
  api_token = "<API Token>" # required for Circonus API, not required for direct IRONdb
  # statsd_interval is the interval at which Circonus is receiving StatsD metrics (Default: 10s)
  statsd_interval = 10
  # statsd_aggregations section defines what to do with StatsD
  # aggregations, and which ones to act on
  [circonus.statsd_aggregations]
    remove = true
    agg_list = ["mean","sum","count_ps"]

# Grafana section defines parameters for connecting to Grafana and 
# managing assets within Grafana
[grafana]
  api_token = "<Grafana API Token>"
  anonymous_auth = false # boolean value if Grafana supports anonymous auth, comment out api token if set
  host = "<Grafana Host>" # e.g. "grafana.example.com"
  port = "<Grafana Port>" # optional
  path = "<Grafana Path>" # optional e.g. "grafana.example.com/<path>" include the leading "/"
  src_folder = "<Source Folder>"
  dest_folder = "<Destination Folder>"
  # whether or not to connect with HTTP or HTTPS
  secure = false
  # name of the configured Circonus datasource
  circonus_datasource = "<Datasource Name>"
  # list of graphite datasource names to convert, leave empty to convert all
  graphite_datasources = ["ds1", "ds2", "ds3"]
  # the below setting nulls out alerts on panels
  no_alerts = false
```