package workspacetemplate

import (
	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/gofiber/fiber/v2"
)

type createWorkspaceTemplate200Response struct {
	doc apitypes.WorkflowTemplateDocument
}

func (response createWorkspaceTemplate200Response) VisitCreateWorkspaceTemplateResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(200)
	return ctx.JSON(response.doc)
}

type getWorkspaceTemplate200Response struct {
	doc apitypes.WorkflowTemplateDocument
}

func (response getWorkspaceTemplate200Response) VisitGetWorkspaceTemplateResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(200)
	return ctx.JSON(response.doc)
}

type putWorkspaceTemplate200Response struct {
	doc apitypes.WorkflowTemplateDocument
}

func (response putWorkspaceTemplate200Response) VisitPutWorkspaceTemplateResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(200)
	return ctx.JSON(response.doc)
}

type deleteWorkspaceTemplate200Response struct {
	doc apitypes.WorkflowTemplateDocument
}

func (response deleteWorkspaceTemplate200Response) VisitDeleteWorkspaceTemplateResponse(ctx *fiber.Ctx) error {
	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(200)
	return ctx.JSON(response.doc)
}
