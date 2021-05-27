package circonus

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type TranslateResponseBody struct {
	Input string `json:"input"`
	CAQL  string `json:"caql"`
	Error string `json:"error"`
}

type Client struct {
	host       string
	httpClient *http.Client
}

// New creates a new Circonus Client
func New(irondbHost string) (*Client, error) {

	// validate that irondb host was provided
	if irondbHost == "" {
		return nil, errors.New("must provide IRONdb host")
	}
	// return the circonus client
	return &Client{
		httpClient: http.DefaultClient,
		host:       irondbHost + ":8112",
	}, nil
}

// Translate translates a graphite query into a CAQL query
func (c *Client) Translate(graphiteQuery string) (string, error) {

	// set up the url for the HTTP request
	u := genURL(c.host, graphiteQuery)

	// create the HTTP request
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("error creating HTTP request: %v", err)
	}
	// excecute the HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error fetching translation: %v", err)
	}
	defer resp.Body.Close()
	// read the body from the response into []byte
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}
	var translateResp TranslateResponseBody
	err = json.Unmarshal(body, &translateResp)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling translation response: %v", err)
	}
	if translateResp.CAQL == "" || translateResp.Error != "" {
		return "", fmt.Errorf("error translating graphite query: %s", translateResp.Error)
	}
	return translateResp.CAQL, nil
}

func genURL(host, query string) (u *url.URL) {
	u = &url.URL{
		Scheme: "http",
		Host:   host,
		Path:   "/extension/lua/graphite_translate",
	}
	qs := u.Query()
	qs.Set("q", query)
	u.RawQuery = qs.Encode()
	return
}
