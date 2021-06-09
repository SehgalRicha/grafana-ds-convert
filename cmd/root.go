/*
Copyright © 2021 Circonus Circonus Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	_ "embed"
	"fmt"
	"log"
	"os"

	"github.com/circonus/grafana-ds-convert/circonus"
	"github.com/circonus/grafana-ds-convert/grafana"
	"github.com/circonus/grafana-ds-convert/internal/config"
	"github.com/circonus/grafana-ds-convert/internal/config/keys"
	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

//go:embed version.txt
var version string
var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "grafana-ds-convert",
	Short: "Convert Grafana assets in different QLs to CAQL",
	Long: `grafana-ds-convert allows Grafana users to convert assets 
like dashboards and alerts from different supported query languages
to Circonus Analytics Query Language (CAQL).`,
	Run: func(cmd *cobra.Command, args []string) {

		// print version and exit
		if viper.GetBool("version") {
			fmt.Println(version)
			return
		}

		// show configuration and exit
		if viper.GetString(keys.ShowConfig) != "" {
			if err := config.ShowConfig(os.Stdout); err != nil {
				log.Fatalf("error printing config: %v", err)
			}
			return
		}

		// Validate that the required config items are set
		if err := config.Validate(); err != nil {
			log.Fatalf("error validating config: %v", err)
		}

		// Create Grafana API URL
		var url string
		if viper.GetBool(keys.GrafanaTLS) {
			url = fmt.Sprintf("https://%s:%s", viper.GetString(keys.GrafanaHost), viper.GetString(keys.GrafanaPort))
		} else {
			url = fmt.Sprintf("http://%s:%s", viper.GetString(keys.GrafanaHost), viper.GetString(keys.GrafanaPort))
		}

		// create grafana API interface
		gclient := grafana.New(url, viper.GetString(keys.GrafanaAPIToken), viper.GetBool(keys.Debug))

		// create circonus interface
		circ, err := circonus.New(
			viper.GetString(keys.CirconusIRONdbHost),
			viper.GetString(keys.CirconusIRONdbPort),
			viper.GetBool(keys.Debug),
			viper.GetBool(keys.CirconusStatsdAggregationsRemove),
			viper.GetStringSlice(keys.CirconusStatsdAggregationsList),
		)
		if err != nil {
			log.Fatalf("error connecting to circonus: %v", err)
		}

		// execute the translation
		err = gclient.Translate(
			circ,
			viper.GetString(keys.GrafanaSourceFolder),
			viper.GetString(keys.GrafanaDestFolder),
			viper.GetString(keys.GrafanaDatasource),
		)
		if err != nil {
			log.Fatalf("error translating dashboards: %v", err)
		}

	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	//
	// arguments that do not appear in configuration file
	//

	{
		var (
			longOpt     = "config"
			shortOpt    = "c"
			description = "config file (default: $HOME/.grafana-ds-convert.yaml|.json|.toml)"
		)
		rootCmd.Flags().StringVarP(&cfgFile, longOpt, shortOpt, "", description)
	}
	{
		const (
			key         = keys.ShowConfig
			longOpt     = "show-config"
			description = "show config (json|toml|yaml) and exit"
		)
		rootCmd.PersistentFlags().String(key, "", description)
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			log.Fatalf("error showing config: %v", err)
		}
	}
	{
		const (
			key          = keys.ShowVersion
			longOpt      = "version"
			shortOpt     = "v"
			defaultValue = false
			description  = "show version and exit"
		)
		rootCmd.Flags().BoolP(longOpt, shortOpt, defaultValue, description)
		if err := viper.BindPFlag(key, rootCmd.Flags().Lookup(longOpt)); err != nil {
			log.Fatalf("error printing version: %v", err)
		}
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".grafana-ds-convert" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".grafana-ds-convert")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
