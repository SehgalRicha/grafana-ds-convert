package circonus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/circonus-labs/gosnowth"
	"github.com/circonus/grafana-ds-convert/internal/config/defaults"
	"github.com/circonus/grafana-ds-convert/logger"
)

//TranslateResponseBody is a struct representing a response from the graphite translator service
type TranslateResponseBody struct {
	Input string `json:"input"`
	CAQL  string `json:"caql"`
	Error string `json:"error"`
}

//TranslateRequestBody is a struct representing a request to the graphite translator service
type TranslateRequestBody struct {
	Query string `json:"q"`
}

//Client is a Circonus client
type Client struct {
	GraphiteTranslateURL *url.URL
	IRONdbFindTagsURL    *url.URL
	HTTPClient           *http.Client
	Debug                bool
	StatsdAggregations   []string
	StatsdFlushInterval  int
	APIToken             string
	AccountId            int
	Period               int
}

// New creates a new Circonus Client
func New(host, port, apiToken string, accountId int, debug, removeAggs, directIRONdb bool, aggs []string, flush int, period int) (*Client, error) {

	// set up either direct IRONdb or (default) Circonus API URL
	var graphite_u *url.URL
	var findtags_u *url.URL
	if directIRONdb {
		if host == "" || port == "" {
			return nil, errors.New("must provide both IRONdb host and port")
		}
		graphite_u = &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%s", host, port),
			Path:   "/extension/lua/graphite_translate",
		}
		findtags_u = &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%s", host, port),
			Path:   "/find/tags",
		}
		if accountId <= 0 {
			return nil, errors.New("must provide Circonus Account Id")
		}
	} else {
		if host == "" {
			host = defaults.CirconusHost
		}
		if apiToken == "" {
			return nil, errors.New("must provide Circonus API Token")
		}
		graphite_u = &url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "irondb/extension/lua/public/graphite_translate",
		}
		findtags_u = &url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "irondb/find/tags",
		}
	}

	// check if flush interval is set, if not use the default of 10
	if flush == 0 {
		flush = defaults.StatsdFlushInterval
	}

	cli := &Client{
		HTTPClient:           http.DefaultClient,
		GraphiteTranslateURL: graphite_u,
		IRONdbFindTagsURL:    findtags_u,
		Debug:                debug,
		StatsdFlushInterval:  flush,
		APIToken:             apiToken,
		AccountId:            accountId,
		Period:               period,
	}
	if removeAggs {
		cli.StatsdAggregations = aggs
	}
	return cli, nil
}

// Translate translates a graphite query into a CAQL query
func (c *Client) Translate(graphiteQuery string) (string, error) {

	// set up the body for the HTTP request
	query := strings.Replace(graphiteQuery, " ", "", -1)
	t := TranslateRequestBody{
		Query: query,
	}
	reqBody, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	// debug
	if c.Debug {
		logger.PrintJSONBytes(logger.LvlDebug, "Translate Request Body:", reqBody)
	}

	// execute the translation HTTP query
	translateResp, err := c.ExecuteTranslation(reqBody)
	if err != nil {
		return "", err
	}

	// check for statsd aggregations to replace, if found, replace them and add
	// to the CAQL query the correct CAQL function
	if len(c.StatsdAggregations) > 0 {
		// capture the entire graphite:find because we want to replace it's contents later
		r := regexp.MustCompile(`(graphite:find\('[^']+'\))`)
		translateResp.CAQL = r.ReplaceAllStringFunc(translateResp.CAQL, c.HandleStatsdAggregations)
	}

	return translateResp.CAQL, nil
}

// ExecuteTranslation handles the HTTP request for the translation
func (c *Client) ExecuteTranslation(b []byte) (*TranslateResponseBody, error) {

	// build the request
	reqBody := bytes.NewBuffer(b)
	req, err := http.NewRequest("POST", c.GraphiteTranslateURL.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %v", err)
	}

	// set API Token and other required headers
	if c.APIToken != "" {
		req.Header.Add("X-Circonus-Auth-Token", c.APIToken)
		req.Header.Add("X-Circonus-App-Name", "Grafana Translator")
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	// execute the HTTP request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching translation: %v", err)
	}
	defer resp.Body.Close()
	// read the body from the response into []byte
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	var translateResp TranslateResponseBody
	err = json.Unmarshal(respBody, &translateResp)
	if err != nil {
		// debug
		if c.Debug {
			logger.PrintMarshal(logger.LvlDebug, "Translate Response Body:", translateResp)
		}
		return nil, fmt.Errorf("error unmarshaling translation response: %v", err)
	}
	if translateResp.CAQL == "" {
		// debug
		if c.Debug {
			logger.PrintMarshal(logger.LvlDebug, "Translate Response Body:", translateResp)
		}
		return nil, fmt.Errorf("error translating graphite query: null CAQL string")
	}
	if translateResp.Error != "" {
		// debug
		if c.Debug {
			logger.PrintMarshal(logger.LvlDebug, "Translate Response Body:", translateResp)
		}
		return nil, fmt.Errorf("error translating graphite query: %s", translateResp.Error)
	}
	// debug
	if c.Debug {
		logger.PrintMarshal(logger.LvlDebug, "Translate Response Body:", translateResp)
	}
	return &translateResp, nil
}

func (c *Client) IRONdbFindTags(metricSearchPattern string) ([]gosnowth.FindTagsItem, error) {

	c.IRONdbFindTagsURL.RawQuery = "query=and(__name:[graphite]" + metricSearchPattern + ")"

	req, err := http.NewRequest("GET", c.IRONdbFindTagsURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %v", err)
	}

	// set API Token and other required headers
	if c.APIToken != "" {
		req.Header.Add("X-Circonus-Auth-Token", c.APIToken)
		req.Header.Add("X-Circonus-App-Name", "Grafana Translator")
		if c.AccountId > 0 {
			req.Header.Add("X-Circonus-Account-Id", strconv.FormatInt(int64(c.AccountId), 10))
		}
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	// execute the HTTP request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching find/tags: %v", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error find/tags returned code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	// read the body from the response into []byte
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading find/tags response body: %v", err)
	}
	var findtagsResp []gosnowth.FindTagsItem
	err = json.Unmarshal(respBody, &findtagsResp)
	if err != nil {
		// debug
		if c.Debug {
			logger.PrintMarshal(logger.LvlDebug, "Find Tags Response Body:", findtagsResp)
		}
		return nil, fmt.Errorf("error unmarshaling find tags response: %v", err)
	}
	if c.Debug {
		logger.PrintMarshal(logger.LvlDebug, "Translate Response Body:", findtagsResp)
	}
	return findtagsResp, nil
}

// HandleStatsdAggregations will append the correct CAQL
func (c *Client) HandleStatsdAggregations(s string) string {
	r := regexp.MustCompile(`graphite:find\('([^']+)'\)`)
	metricName := r.FindStringSubmatch(s)
	// take metric name, and substitute $grafana variables into *
	regGrafanaSquash := regexp.MustCompile(`\$[^.]+`)
	metricSearchPattern := regGrafanaSquash.ReplaceAll([]byte(metricName[1]), []byte("*"))

	// query /find/tags for the metric name pattern
	findtagsResponseSlice, err := c.IRONdbFindTags(string(metricSearchPattern))
	if err != nil {
		matched, matcherr := regexp.Match(`: 400$`, []byte(err.Error()))
		if matcherr != nil || !matched {
			logger.Printf(logger.LvlError, err.Error())
		} else {
			logger.Printf(logger.LvlWarning, err.Error())
		}
		logger.Printf(logger.LvlWarning, "Above means we cannot validate '%s'. Translating based on last part of name omly.", s)
		// Keep going, this isn't fatal
	}
	statsdType := ""
	if 0 == len(findtagsResponseSlice) {
		// if no results error out out and continue
		logger.Printf(logger.LvlWarning, "Pattern %s has no metrics inside of Circonus IRONdb", metricSearchPattern)
		// Keep going, this isn't fatal
	} else {
		for _, findTagsMetric := range findtagsResponseSlice {
			buf := bytes.NewBufferString(findTagsMetric.MetricName)
			p := gosnowth.NewMetricParser(buf)
			aMetric, err := p.Parse()
			if err != nil {
				// This shouldn't be possible.  IRONdb gave us back something we cannot parse?
				logger.Printf(logger.LvlError, "Unable to parse findTagsMetric value from IRONdb?! %v", err)
				continue
			}
			// get the statsd_type from the tags
			for _, tag := range aMetric.StreamTags {
				if tag.Category == "statsd_type" {
					if statsdType == "" {
						statsdType = tag.Value
					} else if statsdType != tag.Value {
						// we have non-matching types, and cannot make a determination.
						logger.Printf(logger.LvlWarning, "Pattern %s matches more than one statsd_type value.  Cannot assume a type.", metricSearchPattern)
						statsdType = "UNKNOWN"
						break // no point in continuing
					}
				}
			}
		}
	}

	splits := strings.Split(metricName[1], ".")
	aggNode := splits[len(splits)-1]
	if contains(c.StatsdAggregations, aggNode) {
		appendCAQL := getAppendCAQL(aggNode, c.Period)
		newName := ""
		if statsdType == "count" {
			newName = metricName[1]
			logger.Printf(logger.LvlInfo, "%s identified as a statsd_type count.  Keeping raw name.", newName)
		} else {
			splits = splits[:len(splits)-1]
			newName = strings.Join(splits, ".")
		}
		return fmt.Sprintf("graphite:find:histogram('%s') | %s", newName, appendCAQL)
	}
	// if we're still here, we could still have a statsd_type that we need to replace, but the name doesn't match
	if contains(c.StatsdAggregations, statsdType) {
		appendCAQL := getAppendCAQL(statsdType, c.Period)
		newName := metricName[1]
		logger.Printf(logger.LvlInfo, "%s identified as a statsd_type %s.  Keeping raw name.", newName, statsdType)
		return fmt.Sprintf("graphite:find:histogram('%s') | %s", newName, appendCAQL)
	}

	return s
}

func contains(s []string, t string) bool {
	for _, m := range s {
		if m == t {
			return true
		}
	}
	return false
}

func getAppendCAQL(statsdAgg string, period int) string {
	switch statsdAgg {
	case "sum":
		if period > 0 {
			return fmt.Sprintf("histogram:sum(period=%ds)", period)
		} else {
			return "histogram:sum()"
		}
	case "count":
		return "histogram:count()"
	case "mean":
		return "histogram:mean()"
	case "lower":
		return "histogram:min()"
	case "median":
		return "histogram:percentile(50)"
	case "upper":
		return "histogram:max()"
	case "upper_90":
		return "histogram:percentile(90)"
	case "upper_95":
		return "histogram:percentile(95)"
	case "upper_99":
		return "histogram:percentile(99)"
	case "count_ps":
		return "histogram:rate(period=1s)"
	case "std":
		return "histogram:stddev()"
	}
	if strings.Contains(statsdAgg, "_") {
		split := strings.Split(statsdAgg, "_")
		switch split[0] {
		case "mean":
			return fmt.Sprintf("histogram:clamp_percentile(0,%s) | histogram:mean()", split[1])
		case "sum":
			return fmt.Sprintf("histogram:clamp_percentile(0,%s) | histogram:sum()", split[1])
		case "upper":
			return fmt.Sprintf("histogram:percentile(%s)", split[1])
		}
	}
	return ""
}
