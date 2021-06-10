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
	"strings"

	"github.com/circonus/grafana-ds-convert/debug"
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
	URL                *url.URL
	HTTPClient         *http.Client
	Debug              bool
	StatsdAggregations []string
}

// New creates a new Circonus Client
func New(irondbHost, irondbPort string, debug, removeAggs bool, aggs []string) (*Client, error) {

	// validate that irondb host was provided
	if irondbHost == "" {
		return nil, errors.New("must provide IRONdb host")
	}

	// set up URL
	u := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%s", irondbHost, irondbPort),
		Path:   "/extension/lua/graphite_translate",
	}

	if removeAggs {
		// return the circonus client
		return &Client{
			HTTPClient:         http.DefaultClient,
			URL:                u,
			Debug:              debug,
			StatsdAggregations: aggs,
		}, nil
	}
	// return the circonus client
	return &Client{
		HTTPClient: http.DefaultClient,
		URL:        u,
		Debug:      debug,
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
		r := regexp.MustCompile(`graphite:find\('([a-zA-Z\.\*0-9]+)`)
		translateResp.CAQL = r.ReplaceAllStringFunc(translateResp.CAQL, c.RemoveAggs)
	}
	return translateResp.CAQL, nil
}

// ExecuteTranslation handles the HTTP request for the translation
func (c *Client) ExecuteTranslation(b []byte) (*TranslateResponseBody, error) {
	reqBody := bytes.NewBuffer(b)
	req, err := http.NewRequest("POST", c.URL.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %v", err)
	}
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
		return nil, fmt.Errorf("error unmarshaling translation response: %v", err)
	}
	if translateResp.CAQL == "" {
		return nil, fmt.Errorf("error translating graphite query: null CAQL string")
	}
	if translateResp.Error != "" {
		return nil, fmt.Errorf("error translating graphite query: %s", translateResp.Error)
	}
	// debug
	if c.Debug {
		debug.PrintMarshal("Translate Response Body:", translateResp)
	}
	return &translateResp, nil
}

//RemoveAggs removes the StatsD aggregations from the metric name
func (c *Client) RemoveAggs(s string) string {
	splits := strings.Split(s, ".")
	if contains(c.StatsdAggregations, splits[len(splits)-1]) {
		splits = splits[:len(splits)-1]
		return strings.Join(splits, ".")
	}
	return strings.Join(splits, ".")
}

func contains(s []string, t string) bool {
	for _, m := range s {
		if m == t {
			return true
		}
	}
	return false
}
