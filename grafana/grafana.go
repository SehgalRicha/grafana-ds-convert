package grafana

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/circonus/grafana-ds-convert/circonus"
	"github.com/circonus/grafana-ds-convert/debug"
	"github.com/rizkybiz/sdk"
)

//Grafana is a struct that holds the sdk client and other properties
type Grafana struct {
	Client         *sdk.Client
	CirconusClient *circonus.Client
	Debug          bool
}

// New creates a new Grafana
func New(url, apikey string, debug bool, c *circonus.Client) Grafana {
	return Grafana{
		Client:         sdk.NewClient(url, apikey, http.DefaultClient, debug),
		Debug:          debug,
		CirconusClient: c,
	}
}

// Translate is the main function which performs dashboard translations
func (g Grafana) Translate(sourceFolder, destFolder, datasource string) error {

	if g.Debug {
		log.Printf("Translation URL: %s", g.CirconusClient.URL.String())
	}

	// get grafana source and destination folders
	var srcFolder sdk.FoundBoard
	var dstFolder sdk.FoundBoard
	foundFolders, err := g.Client.Search(context.Background(), sdk.SearchType(sdk.SearchTypeFolder))
	if err != nil {
		return fmt.Errorf("error fetching grafana dashboard folders: %v", err)
	}
	for _, folder := range foundFolders {
		if folder.Title == sourceFolder {
			srcFolder = folder
		} else if folder.Title == destFolder {
			dstFolder = folder
		}
	}
	if srcFolder.Title == "" {
		return errors.New("no match found for Grafana source folder")
	}
	if dstFolder.Title == "" {
		return errors.New("no match found for Grafana destination folder")
	}
	// debug
	if g.Debug {
		debug.PrintMarshal("Found source folder:", srcFolder)
		debug.PrintMarshal("Found destination folder:", destFolder)
	}

	// get dashboards within found folder
	foundBoards, err := g.Client.Search(context.Background(), sdk.SearchType(sdk.SearchTypeDashboard), sdk.SearchFolderID(int(srcFolder.ID)))
	if err != nil {
		return fmt.Errorf("error fetching dashboards within folder: %v", err)
	}
	// debug
	if g.Debug {
		debug.PrintMarshal("Dashboards from Folder:", foundBoards)
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

	// start the dashboard conversion
	err = g.ConvertDashboards(boards, datasource, dstFolder)
	if err != nil {
		return err
	}
	log.Println("successfully converted dashboards, exiting.")
	return nil
}

// ConvertDashboards iterates through dashboards and converts
// their panels to use CAQL as data queries
func (g Grafana) ConvertDashboards(boards []sdk.Board, datasource string, destinationFolder sdk.FoundBoard) error {
	// loop through dashboards and their panels, translating "targetFull" or "target"
	for _, board := range boards {
		if len(board.Panels) >= 1 {
			// loop through panels and process them
			err := g.ConvertPanels(board.Panels, datasource)
			if err != nil {
				log.Println(fmt.Errorf("error:\n Dashboard: %s\n %v", board.Title, err))
			}
		}
		if g.Debug {
			debug.PrintMarshal("Converted Dashboard:", board)
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
			log.Println(fmt.Errorf("error:\n Dashboard: %s\n %v", board.Title, err))
		}
		if g.Debug {
			debug.PrintMarshal("Create Dashboard Response:", sm)
		}
	}
	return nil
}

// ConvertPanels converts individual panels of a dashboard to use CAQL as data queries
func (g Grafana) ConvertPanels(p []*sdk.Panel, datasource string) error {
	for _, panel := range p {
		panel.Datasource = &datasource
		targets := panel.GetTargets()
		if targets == nil {
			continue
		}
		if len(*targets) >= 1 {
			for _, target := range *targets {
				if target.TargetFull != "" {
					newTargetStr, err := g.CirconusClient.Translate(target.TargetFull)
					if err != nil {
						return fmt.Errorf("%v:\n  Panel: %s\n  Target: %s", err, panel.Title, target.TargetFull)
					}
					target.Query = newTargetStr
					target.Target = ""
					target.TargetFull = ""
					panel.SetTarget(&target)
					continue
				} else {
					newTargetStr, err := g.CirconusClient.Translate(target.Target)
					if err != nil {
						return fmt.Errorf("%v:\n  Panel: %s\n  Target: %s", err, panel.Title, target.Target)
					}
					target.Query = newTargetStr
					target.Target = ""
					panel.SetTarget(&target)
				}
			}
		}
	}
	return nil
}
