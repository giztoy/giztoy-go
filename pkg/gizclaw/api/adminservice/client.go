package adminservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type Client struct {
	api ClientWithResponsesInterface
}

type ClientError struct {
	Path       string
	StatusCode int
	Body       []byte
	Payload    *ErrorResponse
}

func (e *ClientError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Payload != nil && e.Payload.Error.Code != "" && e.Payload.Error.Message != "" {
		return fmt.Sprintf("%s: %s: %s (%d)", e.Path, e.Payload.Error.Code, e.Payload.Error.Message, e.StatusCode)
	}
	if e.Payload != nil && e.Payload.Error.Message != "" {
		return fmt.Sprintf("%s: %s (%d)", e.Path, e.Payload.Error.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s: status %d", e.Path, e.StatusCode)
}

func NewClient(server string, opts ...ClientOption) (*Client, error) {
	api, err := NewClientWithResponses(server, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{api: api}, nil
}

func (c *Client) ListDepots(ctx context.Context) (DepotList, error) {
	resp, err := c.api.ListDepotsWithResponse(ctx)
	if err != nil {
		return DepotList{}, err
	}
	return decodeJSONResponse[DepotList](resp.StatusCode(), resp.Body, "/firmwares", resp.JSON200)
}

func (c *Client) GetDepot(ctx context.Context, depot DepotName) (Depot, error) {
	resp, err := c.api.GetDepotWithResponse(ctx, depot)
	if err != nil {
		return Depot{}, err
	}
	return decodeJSONResponse[Depot](resp.StatusCode(), resp.Body, "/firmwares/{depot}", resp.JSON200)
}

func (c *Client) PutDepotInfo(ctx context.Context, depot DepotName, body PutDepotInfoJSONRequestBody) (Depot, error) {
	resp, err := c.api.PutDepotInfoWithResponse(ctx, depot, body)
	if err != nil {
		return Depot{}, err
	}
	return decodeJSONResponse[Depot](resp.StatusCode(), resp.Body, "/firmwares/{depot}", resp.JSON200)
}

func (c *Client) GetChannel(ctx context.Context, depot DepotName, channel Channel) (DepotRelease, error) {
	resp, err := c.api.GetChannelWithResponse(ctx, depot, channel)
	if err != nil {
		return DepotRelease{}, err
	}
	return decodeJSONResponse[DepotRelease](resp.StatusCode(), resp.Body, "/firmwares/{depot}/{channel}", resp.JSON200)
}

func (c *Client) PutChannel(ctx context.Context, depot DepotName, channel Channel, body io.Reader) (DepotRelease, error) {
	resp, err := c.api.PutChannelWithBodyWithResponse(ctx, depot, channel, "application/octet-stream", body)
	if err != nil {
		return DepotRelease{}, err
	}
	return decodeJSONResponse[DepotRelease](resp.StatusCode(), resp.Body, "/firmwares/{depot}/{channel}", resp.JSON200)
}

func (c *Client) ReleaseDepot(ctx context.Context, depot DepotName) (Depot, error) {
	resp, err := c.api.ReleaseDepotWithResponse(ctx, depot)
	if err != nil {
		return Depot{}, err
	}
	return decodeJSONResponse[Depot](resp.StatusCode(), resp.Body, "/firmwares/{depot}:release", resp.JSON200)
}

func (c *Client) RollbackDepot(ctx context.Context, depot DepotName) (Depot, error) {
	resp, err := c.api.RollbackDepotWithResponse(ctx, depot)
	if err != nil {
		return Depot{}, err
	}
	return decodeJSONResponse[Depot](resp.StatusCode(), resp.Body, "/firmwares/{depot}:rollback", resp.JSON200)
}

func decodeJSONResponse[T any](statusCode int, body []byte, path string, decoded *T) (T, error) {
	var out T
	if statusCode == 0 {
		return out, fmt.Errorf("%s: empty response", path)
	}
	if statusCode/100 != 2 {
		return out, newClientError(path, statusCode, body)
	}
	if decoded != nil {
		return *decoded, nil
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, err
	}
	return out, nil
}

func newClientError(path string, statusCode int, body []byte) error {
	payload := new(ErrorResponse)
	if err := json.Unmarshal(body, payload); err == nil && payload.Error.Code != "" {
		return &ClientError{Path: path, StatusCode: statusCode, Body: body, Payload: payload}
	}
	return &ClientError{Path: path, StatusCode: statusCode, Body: body}
}
