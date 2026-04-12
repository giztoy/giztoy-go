package gearservice

import (
	"context"
	"encoding/json"
	"fmt"
)

type GearClient struct {
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

func NewGearClient(server string, opts ...ClientOption) (*GearClient, error) {
	api, err := NewClientWithResponses(server, opts...)
	if err != nil {
		return nil, err
	}
	return &GearClient{api: api}, nil
}

func (c *GearClient) ListGears(ctx context.Context) (RegistrationList, error) {
	resp, err := c.api.ListGearsWithResponse(ctx)
	if err != nil {
		return RegistrationList{}, err
	}
	return decodeJSONResponse[RegistrationList](resp.StatusCode(), resp.Body, "/gears", resp.JSON200)
}

func (c *GearClient) ListByCertification(ctx context.Context, certType GearCertificationType, authority GearCertificationAuthority, id string) (RegistrationList, error) {
	resp, err := c.api.ListByCertificationWithResponse(ctx, certType, authority, id)
	if err != nil {
		return RegistrationList{}, err
	}
	return decodeJSONResponse[RegistrationList](resp.StatusCode(), resp.Body, "/gears/certification/{type}/{authority}/{id}", resp.JSON200)
}

func (c *GearClient) ListByFirmware(ctx context.Context, depot string, channel GearFirmwareChannel) (RegistrationList, error) {
	resp, err := c.api.ListByFirmwareWithResponse(ctx, depot, channel)
	if err != nil {
		return RegistrationList{}, err
	}
	return decodeJSONResponse[RegistrationList](resp.StatusCode(), resp.Body, "/gears/firmware/{depot}/{channel}", resp.JSON200)
}

func (c *GearClient) ResolveByIMEI(ctx context.Context, tac, serial string) (PublicKeyResponse, error) {
	resp, err := c.api.ResolveByIMEIWithResponse(ctx, tac, serial)
	if err != nil {
		return PublicKeyResponse{}, err
	}
	return decodeJSONResponse[PublicKeyResponse](resp.StatusCode(), resp.Body, "/gears/imei/{tac}/{serial}", resp.JSON200)
}

func (c *GearClient) ListByLabel(ctx context.Context, key, value string) (RegistrationList, error) {
	resp, err := c.api.ListByLabelWithResponse(ctx, key, value)
	if err != nil {
		return RegistrationList{}, err
	}
	return decodeJSONResponse[RegistrationList](resp.StatusCode(), resp.Body, "/gears/label/{key}/{value}", resp.JSON200)
}

func (c *GearClient) ResolveBySN(ctx context.Context, sn string) (PublicKeyResponse, error) {
	resp, err := c.api.ResolveBySNWithResponse(ctx, sn)
	if err != nil {
		return PublicKeyResponse{}, err
	}
	return decodeJSONResponse[PublicKeyResponse](resp.StatusCode(), resp.Body, "/gears/sn/{sn}", resp.JSON200)
}

func (c *GearClient) DeleteGear(ctx context.Context, publicKey PublicKey) (Registration, error) {
	resp, err := c.api.DeleteGearWithResponse(ctx, publicKey)
	if err != nil {
		return Registration{}, err
	}
	return decodeJSONResponse[Registration](resp.StatusCode(), resp.Body, "/gears/{publicKey}", resp.JSON200)
}

func (c *GearClient) GetGear(ctx context.Context, publicKey PublicKey) (Registration, error) {
	resp, err := c.api.GetGearWithResponse(ctx, publicKey)
	if err != nil {
		return Registration{}, err
	}
	return decodeJSONResponse[Registration](resp.StatusCode(), resp.Body, "/gears/{publicKey}", resp.JSON200)
}

func (c *GearClient) GetGearConfig(ctx context.Context, publicKey PublicKey) (Configuration, error) {
	resp, err := c.api.GetGearConfigWithResponse(ctx, publicKey)
	if err != nil {
		return Configuration{}, err
	}
	return decodeJSONResponse[Configuration](resp.StatusCode(), resp.Body, "/gears/{publicKey}/config", resp.JSON200)
}

func (c *GearClient) PutGearConfig(ctx context.Context, publicKey PublicKey, body PutGearConfigJSONRequestBody) (Configuration, error) {
	resp, err := c.api.PutGearConfigWithResponse(ctx, publicKey, body)
	if err != nil {
		return Configuration{}, err
	}
	return decodeJSONResponse[Configuration](resp.StatusCode(), resp.Body, "/gears/{publicKey}/config", resp.JSON200)
}

func (c *GearClient) GetGearInfo(ctx context.Context, publicKey PublicKey) (DeviceInfo, error) {
	resp, err := c.api.GetGearInfoWithResponse(ctx, publicKey)
	if err != nil {
		return DeviceInfo{}, err
	}
	return decodeJSONResponse[DeviceInfo](resp.StatusCode(), resp.Body, "/gears/{publicKey}/info", resp.JSON200)
}

func (c *GearClient) GetGearOTA(ctx context.Context, publicKey PublicKey) (OTASummary, error) {
	resp, err := c.api.GetGearOTAWithResponse(ctx, publicKey)
	if err != nil {
		return OTASummary{}, err
	}
	return decodeJSONResponse[OTASummary](resp.StatusCode(), resp.Body, "/gears/{publicKey}/ota", resp.JSON200)
}

func (c *GearClient) GetGearRuntime(ctx context.Context, publicKey PublicKey) (Runtime, error) {
	resp, err := c.api.GetGearRuntimeWithResponse(ctx, publicKey)
	if err != nil {
		return Runtime{}, err
	}
	return decodeJSONResponse[Runtime](resp.StatusCode(), resp.Body, "/gears/{publicKey}/runtime", resp.JSON200)
}

func (c *GearClient) ApproveGear(ctx context.Context, publicKey PublicKey, body ApproveGearJSONRequestBody) (Registration, error) {
	resp, err := c.api.ApproveGearWithResponse(ctx, publicKey, body)
	if err != nil {
		return Registration{}, err
	}
	return decodeJSONResponse[Registration](resp.StatusCode(), resp.Body, "/gears/{publicKey}:approve", resp.JSON200)
}

func (c *GearClient) BlockGear(ctx context.Context, publicKey PublicKey) (Registration, error) {
	resp, err := c.api.BlockGearWithResponse(ctx, publicKey)
	if err != nil {
		return Registration{}, err
	}
	return decodeJSONResponse[Registration](resp.StatusCode(), resp.Body, "/gears/{publicKey}:block", resp.JSON200)
}

func (c *GearClient) RefreshGear(ctx context.Context, publicKey PublicKey) (RefreshResult, error) {
	resp, err := c.api.RefreshGearWithResponse(ctx, publicKey)
	if err != nil {
		return RefreshResult{}, err
	}
	return decodeJSONResponse[RefreshResult](resp.StatusCode(), resp.Body, "/gears/{publicKey}:refresh", resp.JSON200)
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
