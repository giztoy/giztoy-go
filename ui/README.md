# GizClaw UI

This directory contains the shared frontend source for the embedded `admin` and `play` web UIs.

- `apps/admin`: admin UI entrypoint
- `apps/play`: play UI entrypoint
- `packages`: generated TypeScript SDKs from `../api/*.json`
- `packages/adminservice`: generated SDK for `admin_service.json`
- `packages/serverpublic`: generated SDK for `server_public.json`
- `packages/peerpublic`: generated SDK for `peer_public.json`
- `packages/gearservice`: generated SDK for `gear_service.json`

Each app owns its own generated assets and embedded Go package:

- `go generate ./ui/apps/admin`
- `go generate ./ui/apps/play`

OpenAPI TypeScript SDKs are generated with:

- `npm run gen:sdk`

`npm run gen:sdk` reads OpenAPI JSON inputs from `../api/*.json`. The generated SDKs under `packages/*` are committed, so UI builds do not need to re-run SDK generation unless those specs change.
