package circonus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type TranslateResponseBody struct {
	Input string `json:"input"`
	CAQL  string `json:"caql"`
	Error string `json:"error"`
}

type TranslateRequestBody struct {
	Query string `json:"q"`
}

type Client struct {
	URL        *url.URL
	HTTPClient *http.Client
	Debug      bool
}

// New creates a new Circonus Client
func New(irondbHost, irondbPort string, debug bool) (*Client, error) {

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

	// return the circonus client
	return &Client{
		HTTPClient: http.DefaultClient,
		URL:        u,
		Debug:      debug,
	}, nil
}

// Translate translates a graphite query into a CAQL query
func (c *Client) Translate(graphiteQuery string, removeAggs bool, aggs []string) (string, error) {

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
		log.Printf("Translate URL: %s\n", c.URL.String())
		log.Println("Request Body:")
		log.Println(string(reqBody))
	}

	// create the HTTP request
	req, err := http.NewRequest("POST", c.URL.String(), bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("error creating HTTP request: %v", err)
	}
	// excecute the HTTP request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error fetching translation: %v", err)
	}
	defer resp.Body.Close()
	// read the body from the response into []byte
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}
	// debug
	if c.Debug {
		log.Println(string(respBody))
	}
	var translateResp TranslateResponseBody
	err = json.Unmarshal(respBody, &translateResp)
	// debug
	if c.Debug {
		log.Println("Translation Response:")
		pp, _ := json.MarshalIndent(translateResp, "", "    ")
		fmt.Println(string(pp))
	}
	if err != nil {
		return "", fmt.Errorf("error unmarshaling translation response: %v", err)
	}
	if translateResp.CAQL == "" || translateResp.Error != "" {
		return "", fmt.Errorf("error translating graphite query: %s", translateResp.Error)
	}
	if removeAggs {
		r := regexp.MustCompile(`graphite:find\('([a-zA-Z\.\*0-9]+)`)
		fmt.Printf("Found Capture Groups: %#v\n", r.FindStringSubmatch(translateResp.CAQL))
	}
	return translateResp.CAQL, nil
}
