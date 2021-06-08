package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/circonus/grafana-ds-convert/circonus"
	"github.com/circonus/grafana-ds-convert/internal/config/keys"
	"github.com/grafana-tools/sdk"
	"github.com/spf13/viper"
)

type Grafana struct {
	Client *sdk.Client
	Debug  bool
}

func New(url, apikey string, debug bool) Grafana {
	return Grafana{
		Client: sdk.NewClient(url, apikey, http.DefaultClient),
		Debug:  debug,
	}
}

func (g Grafana) Translate(circ *circonus.Client, sourceFolder, destFolder, datasource string, removeAggs bool, aggs []string) error {

	fmt.Println(removeAggs)

	// validate src and dest folders
	if sourceFolder == "" || destFolder == "" {
		return errors.New("must provide source and destination folders")
	}

	// get grafana source folder
	foundBoards, err := g.Client.Search(context.Background(), sdk.SearchType(sdk.SearchTypeFolder), sdk.SearchQuery(viper.GetString(keys.GrafanaSourceFolder)))
	if err != nil {
		return fmt.Errorf("error fetching grafana dashboard folder: %v", err)
	}
	if len(foundBoards) > 1 {
		return fmt.Errorf("found more than one folder, please check folder name")
	}
	// debug
	if g.Debug {
		log.Println("Found folder:")
		pp, _ := json.MarshalIndent(foundBoards, "", "    ")
		fmt.Println(string(pp))
	}

	// get dashboards within found folder
	foundBoards, err = g.Client.Search(context.Background(), sdk.SearchType(sdk.SearchTypeDashboard), sdk.SearchFolderID(int(foundBoards[0].ID)))
	if err != nil {
		return fmt.Errorf("error fetching dashboards within folder: %v", err)
	}
	// debug
	if g.Debug {
		log.Println("Dashboards from Folder:")
		pp, _ := json.MarshalIndent(foundBoards, "", "    ")
		fmt.Println(string(pp))
	}

	// loop through dashboards in the found folder and create an array of them as well as dashboard properties
	var boards []sdk.Board
	// var boardProps []sdk.BoardProperties
	for _, b := range foundBoards {
		brd, _, err := g.Client.GetDashboardByUID(context.Background(), b.UID)
		if err != nil {
			return fmt.Errorf("error fetching dashboard by UID: %v", err)
		}
		boards = append(boards, brd)
		// boardProps = append(boardProps, brdProp)
	}

	// loop through dashboards and their panels, translating "targetFull"
	for _, board := range boards {
		if len(board.Panels) >= 1 {
			for _, panel := range board.Panels {
				panel.Datasource = &datasource
				targets := panel.GetTargets()
				if len(*targets) >= 1 {
					for _, target := range *targets {
						if target.TargetFull != "" {
							newTargetStr, err := circ.Translate(target.TargetFull, removeAggs, aggs)
							if err != nil {
								log.Println(err)
								continue
							}
							target.TargetFull = newTargetStr
							target.Target = ""
							continue
						} else {
							newTargetStr, err := circ.Translate(target.Target, removeAggs, aggs)
							if err != nil {
								log.Println(err)
								continue
							}
							target.Target = newTargetStr
						}
					}
				}
			}
		}
	}
	return nil
}
