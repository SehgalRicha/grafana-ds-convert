// Copyright © 2021 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package defaults contains the default values for configuration options
package defaults

const (
	//
	// Circonus Defaults
	//

	// CirconusHost is the hostname for IRONdb
	CirconusHost = "api.circonus.com"
	// StatsdFlushInterval is the interval at which we're receiving metrics
	StatsdFlushInterval = 10

	//
	// Grafana Defaults
	//

	//GrafanaAPIToken is the token for accessing Grafana
	GrafanaAPIToken = ""
	// GrafanaHost is the host for accessing Grafana
	GrafanaHost = "localhost"
	// GrafanaPort is the port for accessing Grafana
	GrafanaPort = "3000"

	//
	// Misc Defaults
	//

	//Debug is a global setting for turning on debugging
	Debug = false
)
