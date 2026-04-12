package serverpublic

import "github.com/gofiber/fiber/v2"

type getConfig500JSONResponse ErrorResponse

func (response getConfig500JSONResponse) VisitGetConfigResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}

type downloadFirmware500JSONResponse ErrorResponse

func (response downloadFirmware500JSONResponse) VisitDownloadFirmwareResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}

type getInfo500JSONResponse ErrorResponse

func (response getInfo500JSONResponse) VisitGetInfoResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}

type putInfo500JSONResponse ErrorResponse

func (response putInfo500JSONResponse) VisitPutInfoResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}

type getOTA500JSONResponse ErrorResponse

func (response getOTA500JSONResponse) VisitGetOTAResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}

type registerGear500JSONResponse ErrorResponse

func (response registerGear500JSONResponse) VisitRegisterGearResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}
