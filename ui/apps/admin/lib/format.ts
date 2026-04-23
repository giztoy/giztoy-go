import type { DepotRelease } from "../../../packages/adminservice";
import type { DeviceInfo } from "../../../packages/adminservice";

export function formatServerTime(value: number | undefined): string {
  if (value === undefined) {
    return "Server time unavailable";
  }
  return new Date(value).toLocaleString();
}

export function formatRelease(release: DepotRelease | undefined): string {
  return release?.firmware_semver && release.firmware_semver !== "" ? release.firmware_semver : "—";
}

export function hasRelease(release: DepotRelease | undefined): boolean {
  return Boolean(release?.firmware_semver && release.firmware_semver !== "");
}

export function formatShortKey(value: string | undefined): string {
  if (value === undefined || value === "") {
    return "No public key";
  }
  if (value.length <= 18) {
    return value;
  }
  return `${value.slice(0, 10)}...${value.slice(-6)}`;
}

export function formatValue(value: string | undefined): string {
  if (value === undefined || value === "") {
    return "—";
  }
  return isDateTimeLike(value) ? formatDate(value) : value;
}

export function formatDate(value: string | undefined): string {
  if (value === undefined || value === "") {
    return "—";
  }
  const time = Date.parse(value);
  if (Number.isNaN(time)) {
    return value;
  }
  return new Date(time).toLocaleString();
}

function isDateTimeLike(value: string): boolean {
  return value.includes("T") || value.endsWith("Z");
}

export function deviceTitle(info: DeviceInfo | null | undefined, publicKey: string): string {
  return info?.name?.trim() ? info.name : formatShortKey(publicKey);
}
