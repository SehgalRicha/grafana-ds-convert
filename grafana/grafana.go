package grafana

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bdunavant/sdk"
	"github.com/circonus/grafana-ds-convert/circonus"
	"github.com/circonus/grafana-ds-convert/logger"
)

//Grafana is a struct that holds the sdk client and other properties
type Grafana struct {
	Client         *sdk.Client
	CirconusClient *circonus.Client
	Debug          bool
	NoAlerts       bool
}

// New creates a new Grafana
func New(url, apikey string, debug, noAlerts bool, c *circonus.Client) Grafana {
	return Grafana{
		Client:         sdk.NewClient(url, apikey, http.DefaultClient, debug),
		Debug:          debug,
		CirconusClient: c,
		NoAlerts:       noAlerts,
	}
}

// Translate is the main function which performs dashboard translations
func (g Grafana) Translate(sourceFolder, destFolder, circonusDatasource string, graphiteDatasources []string) error {
	// get grafana source and destination folders
	var srcFolder sdk.FoundBoard
	var dstFolder sdk.FoundBoard
	foundFolders, err := g.Client.Search(context.Background(), sdk.SearchType(sdk.SearchTypeFolder))
	if err != nil {
		return fmt.Errorf("error fetching grafana dashboard folders: %w", err)
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
		logger.PrintMarshal(logger.LvlDebug, "Found source folder:", srcFolder)
		logger.PrintMarshal(logger.LvlDebug, "Found destination folder:", destFolder)
	}

	// get dashboards within found folder
	foundBoards, err := g.Client.Search(context.Background(), sdk.SearchType(sdk.SearchTypeDashboard), sdk.SearchFolderID(int(srcFolder.ID)))
	if err != nil {
		return fmt.Errorf("error fetching dashboards within folder: %v", err)
	}
	// debug
	if g.Debug {
		logger.PrintMarshal(logger.LvlDebug, "Dashboards from Folder:", foundBoards)
	}

	// loop through dashboards in the found folder and create an array of them as well as dashboard properties
	var boards []sdk.Board
	for _, b := range foundBoards {
		brd, _, err := g.Client.GetDashboardByUID(context.Background(), b.UID)
		if err != nil {
			return fmt.Errorf("error fetching dashboard by UID: %v", err)
		}
		boards = append(boards, brd)
	}

	// start the dashboard conversion
	err = g.ConvertDashboards(boards, circonusDatasource, dstFolder, graphiteDatasources)
	if err != nil {
		return err
	}
	logger.Printf(logger.LvlInfo, "Successfully converted dashboards, exiting.")
	return nil
}

// ConvertDashboards iterates through dashboards and converts
// their panels to use CAQL as data queries
func (g Grafana) ConvertDashboards(boards []sdk.Board, circonusDatasource string, destinationFolder sdk.FoundBoard, graphiteDatasources []string) error {
	// loop through dashboards and their panels, translating "targetFull" or "target"
	for _, board := range boards {
		logger.Printf(logger.LvlInfo, "Converting Dashboard %d: %s", board.ID, board.Title)
		if len(board.Panels) >= 1 {
			// loop through panels and process them
			err := g.ConvertPanels(board.Panels, circonusDatasource, graphiteDatasources)
			if err != nil {
				logger.Printf(logger.LvlError, "Dashboard: %s %v", board.Title, err)
			}
		}
		if g.Debug {
			logger.PrintMarshal(logger.LvlDebug, "Converted Dashboard: ", board)
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
            logger.Printf(logger.LvlError, "Dashboard: %s : %v", board.Title, err)
		}
		if g.Debug {
			logger.PrintMarshal(logger.LvlDebug, "Create Dashboard Response:", sm)
		}
	}
	return nil
}

// ConvertPanels converts individual panels of a dashboard to use CAQL as data queries
func (g Grafana) ConvertPanels(p []*sdk.Panel, circonusDatasource string, graphiteDatasources []string) error {
	for _, panel := range p {
	    logger.Printf(logger.LvlInfo, "Converting Panel %d: %s", panel.ID, panel.Title)
		if panel.Datasource != nil {
			if len(graphiteDatasources) > 0 && !contains(graphiteDatasources, *panel.Datasource) {
				continue
			}
		}
		panel.Datasource = &circonusDatasource
		targets := panel.GetTargets()
		if targets == nil {
			continue
		}
		if g.NoAlerts && panel.Alert != nil {
			panel.Alert = nil
		}
		if len(*targets) >= 1 {
			for _, target := range *targets {
				target.QueryType = "caql"
				if target.TargetFull != "" {
					newTargetStr, err := g.CirconusClient.Translate(target.TargetFull)
					if err != nil {
						logger.Printf(logger.LvlError, "%v:  Panel: %s Target: %s", err, panel.Title, target.TargetFull)
					}
					target.Query = newTargetStr
					target.Target = ""
					target.TargetFull = ""
					panel.SetTarget(&target)
					continue
				} else {
					newTargetStr, err := g.CirconusClient.Translate(target.Target)
					if err != nil {
						logger.Printf(logger.LvlError, "%v: Panel: %s Target: %s", err, panel.Title, target.Target)
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

func contains(strings []string, test string) bool {
	for _, s := range strings {
		if s == test {
			return true
		}
	}
	return false
}
