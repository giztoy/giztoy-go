import { expect, test } from "@playwright/test";

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`missing ${name}`);
  }
  return value;
}

test.describe.configure({ mode: "serial" });

test("admin UI renders real seeded service data", async ({ page }) => {
  const adminURL = requiredEnv("ADMIN_UI_URL");
  const devicePublicKey = requiredEnv("DEVICE_PUBLIC_KEY");

  await page.goto(adminURL);
  const main = page.getByRole("main");
  await expect(main.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  await expect(main.getByText("Devices This Page")).toBeVisible();
  await expect(main.getByText("ui-seed-depot")).toBeVisible();

  await page.goto(`${adminURL}/devices`);
  await expect(main.getByRole("heading", { exact: true, name: "Devices" })).toBeVisible();
  await expect(main.getByRole("row", { name: new RegExp(devicePublicKey) })).toBeVisible();

  await page.goto(`${adminURL}/devices/${encodeURIComponent(devicePublicKey)}`);
  await expect(main.getByRole("heading", { name: "Seeded UI Device" })).toBeVisible();
  await expect(main.getByText("ui-device-sn")).toBeVisible();

  await page.goto(`${adminURL}/firmware`);
  await expect(main.getByRole("heading", { exact: true, name: "Depots" })).toBeVisible();
  await expect(main.getByRole("cell", { name: "ui-seed-depot" })).toBeVisible();
  await expect(main.getByRole("cell", { name: "1.0.0" }).first()).toBeVisible();

  await page.goto(`${adminURL}/providers/credentials`);
  await expect(main.getByRole("cell", { name: "ui-seed-credential" })).toBeVisible();

  await page.goto(`${adminURL}/providers/minimax-tenants`);
  await expect(main.getByRole("cell", { name: "ui-seed-tenant" })).toBeVisible();
  await expect(main.getByText("ui-seed-app")).toBeVisible();

  await page.goto(`${adminURL}/ai/voices`);
  await expect(main.getByRole("cell", { exact: true, name: "ui-seed-voice" })).toBeVisible();
  await expect(main.getByText("Seeded UI Voice")).toBeVisible();

  await page.goto(`${adminURL}/ai/workspace-templates`);
  await expect(main.getByRole("cell", { name: "ui-seed-template" })).toBeVisible();

  await page.goto(`${adminURL}/ai/workspaces`);
  await expect(main.getByRole("cell", { name: "ui-seed-workspace" })).toBeVisible();
});

test("play UI runs real proxied action cards", async ({ page }) => {
  const playURL = requiredEnv("PLAY_UI_URL");

  await page.goto(playURL);
  await expect(page.getByRole("heading", { name: "GizClaw Play" })).toBeVisible();

  await page.getByRole("button", { name: "Run Server Info" }).click();
  await expect(page.getByRole("alert")).toContainText("Server Info loaded successfully.");
  await expect(page.locator("pre")).toContainText("build_commit");

  await page.getByRole("button", { name: "Run Device Info" }).click();
  await expect(page.getByRole("alert")).toContainText("Device Info loaded successfully.");
  await expect(page.locator("pre")).toContainText("Seeded UI Device");

  await page.getByRole("button", { name: "Run Configuration" }).click();
  await expect(page.getByRole("alert")).toContainText("Configuration loaded successfully.");
  await expect(page.locator("pre")).toContainText("stable");

  await page.getByRole("button", { name: "Run OTA Summary" }).click();
  await expect(page.getByRole("alert")).toContainText("OTA Summary loaded successfully.");
  await expect(page.locator("pre")).toContainText("ui-seed-depot");
});
