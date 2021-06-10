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
  irondb_host = "<HOST IP>"
  irondb_port = "<PORT>"
  # statsd_aggregations section defines what to do with StatsD
  # aggregations, and which ones to act on
  [circonus.statsd_aggregations]
    remove = true
    agg_list = ["mean","sum","count_ps"]

# Grafana section defines parameters for connecting to Grafana and 
# managing assets within Grafana
[grafana]
  api_token = "<Grafana API Token>"
  host = "<Grafana Host>"
  port = "<Grafana Port>"
  src_folder = "<Source Folder>"
  dest_folder = "<Destination Folder>"
  # whether or not to connect with HTTP or HTTPS
  secure = false
  # name of the configured Circonus datasource
  datasource = "<Datasource Name>"
```