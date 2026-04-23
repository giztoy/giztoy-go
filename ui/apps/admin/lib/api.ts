import { client as adminClient } from "../../../packages/adminservice/client.gen";
import { client as publicClient } from "../../../packages/serverpublic/client.gen";

export function configureAdminClients(): void {
  adminClient.setConfig({
    baseUrl: "/api/admin",
    responseStyle: "fields",
    throwOnError: false,
  });
  publicClient.setConfig({
    baseUrl: "/api/public",
    responseStyle: "fields",
    throwOnError: false,
  });
}
