package gearservice

import "github.com/gofiber/fiber/v2"

type getGearConfig500JSONResponse ErrorResponse

func (response getGearConfig500JSONResponse) VisitGetGearConfigResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}

type putGearConfig500JSONResponse ErrorResponse

func (response putGearConfig500JSONResponse) VisitPutGearConfigResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}

type getGearInfo500JSONResponse ErrorResponse

func (response getGearInfo500JSONResponse) VisitGetGearInfoResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}

type getGearOTA500JSONResponse ErrorResponse

func (response getGearOTA500JSONResponse) VisitGetGearOTAResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}

type refreshGear500JSONResponse ErrorResponse

func (response refreshGear500JSONResponse) VisitRefreshGearResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(500)
	return ctx.JSON(&response)
}
