package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

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
			logger.Printf(logger.LvlError, "Dashboard %s skipped because it cannot be fetched or parsed. %v", b.UID, err)
			continue
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

		if len(board.Templating.List) > 0 {
			graphite_re := regexp.MustCompile(`(?i)graphite`)
			for _, template := range board.Templating.List {
				if template.Datasource != nil {
					// Only convert graphite datasources
					if !graphite_re.MatchString(*template.Datasource) {
						if g.Debug {
							logger.Printf(logger.LvlDebug, "Skipping template datasource %s since it doesn't match 'graphite'", *template.Datasource)
						}
						continue
					}
					if g.Debug {
						logger.Printf(logger.LvlDebug, "Name: %s Type: %s Datasource %s\nQuery: %s", template.Name, template.Type, *template.Datasource, template.Query)
					}
					if template.Query != nil && len(*template.Query) > 0 {
						*template.Datasource = circonusDatasource
						// strip off the wrapping ""s
						queryStr := *template.Query
						queryStr = queryStr[1 : len(queryStr)-1]
						// Marshal it back into escaped JSON so we can shove it as JSON safely into the RawMessage
						escapedString, err := json.Marshal(string(queryStr))
						if nil != err {
							logger.Printf(logger.LvlError, "Cannot escape variable string: %s", *template.Query)
							continue
						}
						newQueryObj := json.RawMessage(`{"metricFindQuery":` + string(escapedString) + `,"queryType":"graphite style","resultsLimit":500,"tagCategory":""}`)
						if g.Debug {
							logger.Printf(logger.LvlDebug, "variable query before: %s  object: %s", *template.Query, newQueryObj)
						}
						*template.Query = newQueryObj
					}
				}
			}
		}

		if len(board.Panels) >= 1 {
			err := g.ConvertPanels(board.Panels, circonusDatasource, graphiteDatasources)
			if err != nil {
				logger.Printf(logger.LvlError, "Dashboard %d: %s %v", board.ID, board.Title, err)
			}
		} else {
			if g.Debug {
				logger.Printf(logger.LvlDebug, "No top level panels.")
			}
		}
		// Dashboards can also have "rows" and those rows can have their own panels, so look for those as well
		if len(board.Rows) >= 1 {
			foundOne := false
			for _, row := range board.Rows {
				if len(row.Panels) >= 1 {
					foundOne = true
					// board is []*Panel, vs Row is []Panel, so convert it into a slice of *'s so we can pass it in
					var slicearoo []*sdk.Panel
					for i := 0; i < len(row.Panels); i++ {
						slicearoo = append(slicearoo, &row.Panels[i])
					}
					err := g.ConvertPanels(slicearoo, circonusDatasource, graphiteDatasources)
					if err != nil {
						logger.Printf(logger.LvlError, "Dashboard %d: %s error in row panel %v", board.ID, board.Title, err)
					}
				}
			}
			if g.Debug && !foundOne {
				logger.Printf(logger.LvlDebug, "No panels in rows.")
			}
		} else {
			if g.Debug {
				logger.Printf(logger.LvlDebug, "No top level rows.")
			}
		}

		// We are running in local mode so just print the output
		if destinationFolder.Title == "" {
			logger.PrintMarshal(logger.LvlInfo, "Converted Dashboard: ", board)
			return nil
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
				logger.Printf(logger.LvlInfo, "Skipping panel due to datasource type.")
				continue
			}
		}
		if panel.OfType == sdk.RowType && len(panel.Panels) >= 1 {
			// A row panel, can have it's own set of panel inside (who designed this?) loop through THOSE panels and process them
			if g.Debug {
				logger.Printf(logger.LvlDebug, "Panel %d has subpanels, converting those too.", panel.ID)
			}
			// loop through panels and process them
			var slicearoo []*sdk.Panel
			for i := 0; i < len(panel.Panels); i++ {
				slicearoo = append(slicearoo, &panel.Panels[i])
			}
			err := g.ConvertPanels(slicearoo, circonusDatasource, graphiteDatasources)
			if err != nil {
				logger.Printf(logger.LvlError, "Error converting Subpanel inside panel %d : %v", panel.ID, err)
				// skip it and keep going
			}
		} else {
			if g.Debug {
				logger.Printf(logger.LvlDebug, "Panel %d does not have subpanels.", panel.ID)
			}
		}
		targets := panel.GetTargets()
		if targets == nil {
			continue
		}
		if panel.Datasource == nil {
			strcpy := circonusDatasource
			panel.Datasource = &strcpy
		} else {
			*panel.Datasource = circonusDatasource
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
						logger.Printf(logger.LvlError, "Panel: %s Target: %s %v", panel.Title, target.TargetFull, err)
					}
					target.Query = newTargetStr
					target.Target = ""
					target.TargetFull = ""
					panel.SetTarget(&target)
					continue
				} else {
					newTargetStr, err := g.CirconusClient.Translate(target.Target)
					if err != nil {
						logger.Printf(logger.LvlError, "Panel: %s Target: %s %v", panel.Title, target.Target, err)
					}
					target.Query = newTargetStr
					target.Target = ""
					panel.SetTarget(&target)
				}
			}
		} else {
			if g.Debug {
				logger.Printf(logger.LvlInfo, "No targets")
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
