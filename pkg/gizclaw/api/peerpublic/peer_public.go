package peerpublic

import (
	"context"
	"encoding/json"
	"fmt"
)

type Client struct {
	api ClientWithResponsesInterface
}

type ClientError struct {
	Path       string
	StatusCode int
	Body       []byte
}

func (e *ClientError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s: status %d", e.Path, e.StatusCode)
}

func NewClient(doer HttpRequestDoer) (*Client, error) {
	api, err := NewClientWithResponses("http://gizclaw", WithHTTPClient(doer))
	if err != nil {
		return nil, err
	}
	return &Client{api: api}, nil
}

func (c *Client) GetInfo(ctx context.Context) (RefreshInfo, error) {
	resp, err := c.api.GetInfoWithResponse(ctx)
	if err != nil {
		return RefreshInfo{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return decodeResponse[RefreshInfo](resp.StatusCode(), resp.Body, "/info")
}

func (c *Client) GetIdentifiers(ctx context.Context) (RefreshIdentifiers, error) {
	resp, err := c.api.GetIdentifiersWithResponse(ctx)
	if err != nil {
		return RefreshIdentifiers{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return decodeResponse[RefreshIdentifiers](resp.StatusCode(), resp.Body, "/identifiers")
}

func (c *Client) GetVersion(ctx context.Context) (RefreshVersion, error) {
	resp, err := c.api.GetVersionWithResponse(ctx)
	if err != nil {
		return RefreshVersion{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return decodeResponse[RefreshVersion](resp.StatusCode(), resp.Body, "/version")
}

func decodeResponse[T any](statusCode int, body []byte, path string) (T, error) {
	var out T
	if statusCode == 0 {
		return out, fmt.Errorf("peer public http %s: empty response", path)
	}
	if statusCode/100 != 2 {
		return out, &ClientError{Path: path, StatusCode: statusCode, Body: append([]byte(nil), body...)}
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, err
	}
	return out, nil
}
