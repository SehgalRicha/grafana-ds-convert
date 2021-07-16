// Copyright © 2021 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package keys defines the configuration keys used to access viper
package keys

//
// NOTE: adding a key MUST be reflected in the structs defined in config package.
//       the keys must be the same as the encoding tags
//       e.g. `Debug = "debug"` here, corresponds to
//            `json:"debug"` on a struct member
//
const (

	//
	// Grafana
	//

	// API token to access Grafana instance
	GrafanaAPIToken = "grafana.api_token" //nolint:gosec

	// If using anonymous auth
	GrafanaAnonymousAuth = "grafana.anonymous_auth"

	// Host where Grafana is running
	GrafanaHost = "grafana.host"

	// Port where Grafana server is listening
	GrafanaPort = "grafana.port"

	// Optional grafana path
	GrafanaPath = "grafana.path"

	// Use TLS or not when issuing API calls to grafana
	GrafanaTLS = "grafana.secure"

	// Grafana source folder
	GrafanaSourceFolder = "grafana.src_folder"

	// Grafana destination folder
	GrafanaDestFolder = "grafana.dest_folder"

	// Graphite data sources
	GrafanaGraphiteDatasources = "grafana.graphite_datasources"

	// Grafana Circonus Datasource name
	GrafanaCirconusDatasource = "grafana.circonus_datasource"

	//
	// Circonus
	//

	CirconusDirectIRONdb             = "circonus.direct_irondb"
	CirconusAPIToken                 = "circonus.api_token"
	CirconusHost                     = "circonus.host"
	CirconusPort                     = "circonus.port"
	CirconusStatsdFlushInterval      = "circonus.statsd_interval"
	CirconusStatsdAggregationsRemove = "circonus.statsd_aggregations.remove"
	CirconusStatsdAggregationsList   = "circonus.statsd_aggregations.agg_list"

	//
	// Miscellaneous
	//

	// Debug enables debug messages
	Debug = "debug"

	//
	// Informational
	// NOTE: these ARE NOT included in the configuration file as they
	//       trigger display of information and exit
	//

	// ShowConfig - show configuration and exit
	ShowConfig = "show-config"

	// ShowVersion - show version information and exit
	ShowVersion = "version"
)
