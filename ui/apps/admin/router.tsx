import { Navigate, Route, Routes } from "react-router-dom";

import { AdminLayout } from "./layout/AdminLayout";
import { VoicesListPage } from "./pages/ai/VoicesListPage";
import { WorkspaceTemplatesListPage } from "./pages/ai/WorkspaceTemplatesListPage";
import { WorkspacesListPage } from "./pages/ai/WorkspacesListPage";
import { ChannelDetailPage } from "./pages/firmware/ChannelDetailPage";
import { DepotDetailPage } from "./pages/firmware/DepotDetailPage";
import { FirmwareListPage } from "./pages/firmware/FirmwareListPage";
import { FirmwareUploadPage } from "./pages/firmware/FirmwareUploadPage";
import { PeerDetailPage } from "./pages/peers/PeerDetailPage";
import { PeersListPage } from "./pages/peers/PeersListPage";
import { OverviewPage } from "./pages/overview/OverviewPage";
import { CredentialsListPage } from "./pages/providers/CredentialsListPage";
import { MiniMaxTenantsListPage } from "./pages/providers/MiniMaxTenantsListPage";

export function AppRoutes(): JSX.Element {
  return (
    <Routes>
      <Route element={<AdminLayout />} path="/">
        <Route index element={<Navigate replace to="/overview" />} />
        <Route element={<OverviewPage />} path="overview" />
        <Route element={<PeersListPage />} path="peers" />
        <Route element={<PeerDetailPage />} path="peers/:publicKey" />
        <Route element={<FirmwareListPage />} path="firmware" />
        <Route element={<FirmwareUploadPage />} path="firmware/new" />
        <Route element={<DepotDetailPage />} path="firmware/:depot" />
        <Route element={<ChannelDetailPage />} path="firmware/:depot/:channel" />
        <Route element={<CredentialsListPage />} path="providers/credentials" />
        <Route element={<MiniMaxTenantsListPage />} path="providers/minimax-tenants" />
        <Route element={<VoicesListPage />} path="ai/voices" />
        <Route element={<WorkspaceTemplatesListPage />} path="ai/workspace-templates" />
        <Route element={<WorkspacesListPage />} path="ai/workspaces" />
      </Route>
      <Route element={<Navigate replace to="/overview" />} path="*" />
    </Routes>
  );
}
