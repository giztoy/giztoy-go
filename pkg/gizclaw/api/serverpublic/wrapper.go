package serverpublic

import (
	"context"
	"encoding/json"
	"fmt"
)

type PublicClient struct {
	api ClientWithResponsesInterface
}

type ClientError struct {
	Path       string
	StatusCode int
	Body       []byte
	Payload    *ErrorResponse
}

type FirmwareDownload struct {
	Body            []byte
	ContentType     string
	XChecksumMD5    string
	XChecksumSHA256 string
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

func NewPublicClient(server string, opts ...ClientOption) (*PublicClient, error) {
	api, err := NewClientWithResponses(server, opts...)
	if err != nil {
		return nil, err
	}
	return &PublicClient{api: api}, nil
}

func (c *PublicClient) GetConfig(ctx context.Context) (Configuration, error) {
	resp, err := c.api.GetConfigWithResponse(ctx)
	if err != nil {
		return Configuration{}, err
	}
	return decodeJSONResponse[Configuration](resp.StatusCode(), resp.Body, "/config", resp.JSON200)
}

func (c *PublicClient) DownloadFirmware(ctx context.Context, path string) (FirmwareDownload, error) {
	resp, err := c.api.DownloadFirmwareWithResponse(ctx, path)
	if err != nil {
		return FirmwareDownload{}, err
	}
	if resp.StatusCode() != 200 {
		return FirmwareDownload{}, newClientError("/download/firmware/{path}", resp.StatusCode(), resp.Body)
	}
	out := FirmwareDownload{Body: append([]byte(nil), resp.Body...)}
	if resp.HTTPResponse != nil {
		out.ContentType = resp.HTTPResponse.Header.Get("Content-Type")
		out.XChecksumMD5 = resp.HTTPResponse.Header.Get("X-Checksum-MD5")
		out.XChecksumSHA256 = resp.HTTPResponse.Header.Get("X-Checksum-SHA256")
	}
	return out, nil
}

func (c *PublicClient) GetInfo(ctx context.Context) (DeviceInfo, error) {
	resp, err := c.api.GetInfoWithResponse(ctx)
	if err != nil {
		return DeviceInfo{}, err
	}
	return decodeJSONResponse[DeviceInfo](resp.StatusCode(), resp.Body, "/info", resp.JSON200)
}

func (c *PublicClient) PutInfo(ctx context.Context, body PutInfoJSONRequestBody) (DeviceInfo, error) {
	resp, err := c.api.PutInfoWithResponse(ctx, body)
	if err != nil {
		return DeviceInfo{}, err
	}
	return decodeJSONResponse[DeviceInfo](resp.StatusCode(), resp.Body, "/info", resp.JSON200)
}

func (c *PublicClient) GetOTA(ctx context.Context) (OTASummary, error) {
	resp, err := c.api.GetOTAWithResponse(ctx)
	if err != nil {
		return OTASummary{}, err
	}
	return decodeJSONResponse[OTASummary](resp.StatusCode(), resp.Body, "/ota", resp.JSON200)
}

func (c *PublicClient) RegisterGear(ctx context.Context, body RegisterGearJSONRequestBody) (RegistrationResult, error) {
	resp, err := c.api.RegisterGearWithResponse(ctx, body)
	if err != nil {
		return RegistrationResult{}, err
	}
	return decodeJSONResponse[RegistrationResult](resp.StatusCode(), resp.Body, "/register", resp.JSON200)
}

func (c *PublicClient) GetRegistration(ctx context.Context) (Registration, error) {
	resp, err := c.api.GetRegistrationWithResponse(ctx)
	if err != nil {
		return Registration{}, err
	}
	return decodeJSONResponse[Registration](resp.StatusCode(), resp.Body, "/registration", resp.JSON200)
}

func (c *PublicClient) GetRuntime(ctx context.Context) (Runtime, error) {
	resp, err := c.api.GetRuntimeWithResponse(ctx)
	if err != nil {
		return Runtime{}, err
	}
	return decodeJSONResponse[Runtime](resp.StatusCode(), resp.Body, "/runtime", resp.JSON200)
}

func (c *PublicClient) GetServerInfo(ctx context.Context) (ServerInfo, error) {
	resp, err := c.api.GetServerInfoWithResponse(ctx)
	if err != nil {
		return ServerInfo{}, err
	}
	return decodeJSONResponse[ServerInfo](resp.StatusCode(), resp.Body, "/server-info", resp.JSON200)
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
