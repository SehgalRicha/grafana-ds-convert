package cmd

import (
	_ "embed" //embedding the version file
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bdunavant/sdk"
	"github.com/circonus/grafana-ds-convert/circonus"
	"github.com/circonus/grafana-ds-convert/grafana"
	"github.com/circonus/grafana-ds-convert/internal/config"
	"github.com/circonus/grafana-ds-convert/internal/config/keys"
	"github.com/circonus/grafana-ds-convert/logger"
	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

//go:embed version.txt
var version string
var cfgFile string
var localInputFile string

var rootCmd = &cobra.Command{
	Use:   "grafana-ds-convert",
	Short: "Convert Grafana assets in different QLs to CAQL",
	Long: `grafana-ds-convert allows Grafana users to convert assets 
like dashboards and alerts from different supported query languages
to Circonus Analytics Query Language (CAQL).`,
	Run: func(cmd *cobra.Command, args []string) {

		if viper.GetBool("version") {
			fmt.Println(version)
			return
		}

		var queryStrings []string
		var localDashboard *sdk.Board
		if localInputFile != "" {
			if strings.HasSuffix(localInputFile, ".json") {
				// This is a dashboard .json
				boardBytes, err := os.ReadFile(localInputFile)
				if err != nil {
					log.Fatalf("Unable to read from file %s: %v", localInputFile, err)
					return
				}
				err = json.Unmarshal(boardBytes, &localDashboard)
				if err != nil {
					log.Fatalf("Unable to unmarshal dashboard from file %s: %v", localInputFile, err)
				}
				// log.Fatalf("%#v", localDashboard)
			} else {
				queryBytes, err := os.ReadFile(localInputFile)
				if err != nil {
					log.Fatalf("Unable to read from file %s: %v", localInputFile, err)
					return
				}
				queryStrings = strings.Split(string(queryBytes), "\n")
			}
		}

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

		// create circonus interface
		circ, err := circonus.New(
			viper.GetString(keys.CirconusHost),
			viper.GetString(keys.CirconusPort),
			viper.GetString(keys.CirconusAPIToken),
			viper.GetInt(keys.CirconusAccountId),
			viper.GetBool(keys.Debug),
			viper.GetBool(keys.CirconusStatsdAggregationsRemove),
			viper.GetBool(keys.CirconusDirectIRONdb),
			viper.GetStringSlice(keys.CirconusStatsdAggregationsList),
			viper.GetInt(keys.CirconusStatsdFlushInterval),
			viper.GetInt(keys.CirconusStatsdPeriod),
		)
		if err != nil {
			log.Fatalf("error connecting to circonus: %v", err)
		}

		if len(queryStrings) > 0 {
			for _, aGraphiteQuery := range queryStrings {
				q := strings.TrimSpace(aGraphiteQuery)
				if len(q) == 0 {
					continue
				}
				output, err := circ.Translate(q)
				if err != nil {
					logger.Printf(logger.LvlError, "error translating from file: %v", err)
					continue
				}
				fmt.Println(output)
			}
			return
		}

		// Create Grafana API URL
		var url string
		if viper.GetBool(keys.GrafanaTLS) {
			if viper.GetString(keys.GrafanaPort) != "" {
				url = fmt.Sprintf("https://%s:%s%s", viper.GetString(keys.GrafanaHost), viper.GetString(keys.GrafanaPort), viper.GetString(keys.GrafanaPath))
			} else {
				url = fmt.Sprintf("https://%s%s", viper.GetString(keys.GrafanaHost), viper.GetString(keys.GrafanaPath))
			}
		} else {
			if viper.GetString(keys.GrafanaPort) != "" {
				url = fmt.Sprintf("http://%s:%s%s", viper.GetString(keys.GrafanaHost), viper.GetString(keys.GrafanaPort), viper.GetString(keys.GrafanaPath))
			} else {
				url = fmt.Sprintf("http://%s%s", viper.GetString(keys.GrafanaHost), viper.GetString(keys.GrafanaPath))
			}
		}

		// create grafana API interface
		gclient := grafana.New(url, viper.GetString(keys.GrafanaAPIToken), viper.GetBool(keys.Debug), viper.GetBool(keys.GrafanaNoAlerts), circ)

		if localDashboard != nil {
			// translate a single dashboard from file
			boards := []sdk.Board{*localDashboard}
			var emptyDstFolder sdk.FoundBoard
			err = gclient.ConvertDashboards(boards, viper.GetString(keys.GrafanaCirconusDatasource), emptyDstFolder, viper.GetStringSlice(keys.GrafanaGraphiteDatasources))
			if err != nil {
				log.Fatalf("error translating local dashboard: %v", err)
			}
		} else {
			// execute the translation
			err = gclient.Translate(
				viper.GetString(keys.GrafanaSourceFolder),
				viper.GetString(keys.GrafanaDestFolder),
				viper.GetString(keys.GrafanaCirconusDatasource),
				viper.GetStringSlice(keys.GrafanaGraphiteDatasources),
			)
			if err != nil {
				log.Fatalf("error translating dashboards: %v", err)
			}
		}

	},
}

// Execute kicks off the root cmd
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file (default: $HOME/.grafana-ds-convert.yaml|.json|.toml)")
	rootCmd.Flags().StringVarP(&localInputFile, "file", "f", "", "Take a local file to translate.")

	rootCmd.PersistentFlags().String(keys.ShowConfig, "", "show config (json|toml|yaml) and exit")
	if err := viper.BindPFlag(keys.ShowConfig, rootCmd.PersistentFlags().Lookup("show-config")); err != nil {
		logger.Printf(logger.LvlError, "Error binding show-config %v", err)
	}

	rootCmd.Flags().BoolP("version", "v", false, "show version and exit")
	if err := viper.BindPFlag(keys.ShowVersion, rootCmd.Flags().Lookup("version")); err != nil {
		logger.Printf(logger.LvlError, "Error binding show-config %v", err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
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
	if err := viper.ReadInConfig(); err != nil {
		logger.Printf(logger.LvlError, "Error reading config: %v", err)
	}
}
