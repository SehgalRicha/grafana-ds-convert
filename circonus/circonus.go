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

	"github.com/circonus/grafana-ds-convert/debug"
	"github.com/circonus/grafana-ds-convert/internal/config/defaults"
)

//TranslateResponseBody is a struct representing a
//response from the graphite translator service
type TranslateResponseBody struct {
	Input string `json:"input"`
	CAQL  string `json:"caql"`
	Error string `json:"error"`
}

//TranslateRequestBody is a struct representing a
//request to the graphite translator service
type TranslateRequestBody struct {
	Query string `json:"q"`
}

//Client is a Circonus client
type Client struct {
	URL                 *url.URL
	HTTPClient          *http.Client
	Debug               bool
	StatsdAggregations  []string
	StatsdFlushInterval int
	APIToken            string
}

// New creates a new Circonus Client
func New(host, port, apiToken string, debug, removeAggs, directIRONdb bool, aggs []string, flush int) (*Client, error) {

	// set up either direct IRONdb or (default) Circonus API URL
	var u *url.URL
	if directIRONdb {
		if host == "" || port == "" {
			return nil, errors.New("must provide both IRONdb host and port")
		}
		u = &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%s", host, port),
			Path:   "/extension/lua/graphite_translate",
		}
	} else {
		if host == "" {
			host = defaults.CirconusHost
		}
		if apiToken == "" {
			return nil, errors.New("must provide Circonus API Token")
		}
		u = &url.URL{
			Scheme: "https",
			Host:   fmt.Sprintf("%s", host),
			Path:   "irondb/extension/lua/public/graphite_translate",
		}
	}

	// check if flush interval is set, if not use the default of 10
	if flush == 0 {
		flush = defaults.StatsdFlushInterval
	}

	if removeAggs {
		// return the circonus client
		return &Client{
			HTTPClient:          http.DefaultClient,
			URL:                 u,
			Debug:               debug,
			StatsdAggregations:  aggs,
			StatsdFlushInterval: flush,
			APIToken:            apiToken,
		}, nil
	}
	// return the circonus client
	return &Client{
		HTTPClient:          http.DefaultClient,
		URL:                 u,
		Debug:               debug,
		StatsdFlushInterval: flush,
		APIToken:            apiToken,
	}, nil
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
		debug.PrintJSONBytes("Translate Request Body:", reqBody)
	}

	// execute the translation HTTP query
	translateResp, err := c.ExecuteTranslation(reqBody)
	if err != nil {
		return "", err
	}

	// check for statsd aggregations to replace, if found, replace them and add
	// to the CAQL query the correct CAQL function
	if len(c.StatsdAggregations) > 0 {
		r := regexp.MustCompile(`(graphite:find\('[^']+'\))`)
		translateResp.CAQL = r.ReplaceAllStringFunc(translateResp.CAQL, c.HandleStatsdAggregations)
	}

	// add #min_period=X directive for better visualizations
	translateResp.CAQL = fmt.Sprintf("#min_period=%s %s", strconv.Itoa(c.StatsdFlushInterval), translateResp.CAQL)

	return translateResp.CAQL, nil
}

// ExecuteTranslation handles the HTTP request for the translation
func (c *Client) ExecuteTranslation(b []byte) (*TranslateResponseBody, error) {

	// build the request
	reqBody := bytes.NewBuffer(b)
	req, err := http.NewRequest("POST", c.URL.String(), reqBody)
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
			debug.PrintMarshal("Translate Response Body:", translateResp)
		}
		return nil, fmt.Errorf("error unmarshaling translation response: %v", err)
	}
	if translateResp.CAQL == "" {
		// debug
		if c.Debug {
			debug.PrintMarshal("Translate Response Body:", translateResp)
		}
		return nil, fmt.Errorf("error translating graphite query: null CAQL string")
	}
	if translateResp.Error != "" {
		// debug
		if c.Debug {
			debug.PrintMarshal("Translate Response Body:", translateResp)
		}
		return nil, fmt.Errorf("error translating graphite query: %s", translateResp.Error)
	}
	// debug
	if c.Debug {
		debug.PrintMarshal("Translate Response Body:", translateResp)
	}
	return &translateResp, nil
}

// HandleStatsdAggregations will append the correct CAQL
func (c *Client) HandleStatsdAggregations(s string) string {
	r := regexp.MustCompile(`graphite:find\('([^']+)'\)`)
	metricName := r.FindStringSubmatch(s)
	splits := strings.Split(metricName[1], ".")
	aggNode := splits[len(splits)-1]
	if contains(c.StatsdAggregations, aggNode) {
		appendCAQL := getAppendCAQL(aggNode)
		splits = splits[:len(splits)-1]
		return fmt.Sprintf("graphite:find('%s') %s", strings.Join(splits, "."), appendCAQL)
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

func getAppendCAQL(statsdAgg string) string {
	switch statsdAgg {
	case "sum":
		return "histogram:sum()"
	case "count":
		return "histogram:count()"
	case "mean":
		return "histogram:mean()"
	case "lower":
		return "histogram:min()"
	case "upper":
		return "histogram:max()"
	case "count_ps":
		return "histogram:rate(period=1)"
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
