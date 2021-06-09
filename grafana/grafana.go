package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/circonus/grafana-ds-convert/circonus"
	"github.com/grafana-tools/sdk"
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

func (g Grafana) Translate(circ *circonus.Client, sourceFolder, destFolder, datasource string) error {

	// validate src and dest folders
	if sourceFolder == "" || destFolder == "" {
		return errors.New("must provide source and destination folders")
	}

	// get grafana source folder
	foundSrcFolders, err := g.Client.Search(context.Background(), sdk.SearchType(sdk.SearchTypeFolder), sdk.SearchQuery(sourceFolder))
	if err != nil {
		return fmt.Errorf("error fetching grafana dashboard folder: %v", err)
	}
	if len(foundSrcFolders) > 1 {
		return fmt.Errorf("found more than one folder, please check folder name")
	}
	// debug
	if g.Debug {
		log.Println("Found source folder:")
		pp, _ := json.MarshalIndent(foundSrcFolders, "", "    ")
		fmt.Println(string(pp))
	}

	// get grafana destination folder
	foundDestFolders, err := g.Client.Search(context.Background(), sdk.SearchType(sdk.SearchTypeFolder), sdk.SearchQuery(destFolder))
	if err != nil {
		return fmt.Errorf("error fetching grafana dashboard folder: %v", err)
	}
	if len(foundDestFolders) > 1 {
		return fmt.Errorf("found more than one folder, please check folder name")
	}
	// debug
	if g.Debug {
		log.Println("Found source folder:")
		pp, _ := json.MarshalIndent(foundDestFolders, "", "    ")
		fmt.Println(string(pp))
	}
	destinationFolder := foundDestFolders[0]

	// get dashboards within found folder
	foundBoards, err := g.Client.Search(context.Background(), sdk.SearchType(sdk.SearchTypeDashboard), sdk.SearchFolderID(int(foundSrcFolders[0].ID)))
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

	// loop through dashboards and their panels, translating "targetFull" or "target"
	for _, board := range boards {
		if len(board.Panels) >= 1 {
			for _, panel := range board.Panels {
				panel.Datasource = &datasource
				targets := panel.GetTargets()
				if len(*targets) >= 1 {
					for _, target := range *targets {
						if target.TargetFull != "" {
							newTargetStr, err := circ.Translate(target.TargetFull)
							if err != nil {
								log.Println(err)
								continue
							}
							target.Query = newTargetStr
							target.Target = ""
							target.TargetFull = ""
							panel.SetTarget(&target)
							continue
						} else {
							newTargetStr, err := circ.Translate(target.Target)
							if err != nil {
								log.Println(err)
								continue
							}
							target.Query = newTargetStr
							target.Target = ""
							panel.SetTarget(&target)
						}
					}
				}
			}
		}
		if g.Debug {
			pp, _ := json.MarshalIndent(board, "", "    ")
			log.Printf("Converted Dashboard:\n%s", string(pp))
		}
		newBoard := board
		newBoard.ID = 0
		newBoard.UID = ""
		newBoard.Title += " Circonus"
		setDashParams := sdk.SetDashboardParams{
			FolderID:  int(destinationFolder.ID),
			Overwrite: true,
		}
		sm, err := g.Client.SetDashboard(context.Background(), newBoard, setDashParams)
		if err != nil {
			return err
		}
		pp, _ := json.MarshalIndent(sm, "", "    ")
		log.Printf("Create Dashboard Response:\n%s", pp)
	}
	return nil
}
