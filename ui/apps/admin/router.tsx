import { Navigate, Route, Routes } from "react-router-dom";

import { AdminLayout } from "./layout/AdminLayout";
import { ChannelDetailPage } from "./pages/firmware/ChannelDetailPage";
import { DepotDetailPage } from "./pages/firmware/DepotDetailPage";
import { FirmwareListPage } from "./pages/firmware/FirmwareListPage";
import { FirmwareUploadPage } from "./pages/firmware/FirmwareUploadPage";
import { DeviceDetailPage } from "./pages/devices/DeviceDetailPage";
import { DevicesListPage } from "./pages/devices/DevicesListPage";
import { MemoryPage } from "./pages/memory/MemoryPage";
import { OverviewPage } from "./pages/overview/OverviewPage";

export function AppRoutes(): JSX.Element {
  return (
    <Routes>
      <Route element={<AdminLayout />} path="/">
        <Route index element={<Navigate replace to="/overview" />} />
        <Route element={<OverviewPage />} path="overview" />
        <Route element={<DevicesListPage />} path="devices" />
        <Route element={<DeviceDetailPage />} path="devices/:publicKey" />
        <Route element={<FirmwareListPage />} path="firmware" />
        <Route element={<FirmwareUploadPage />} path="firmware/new" />
        <Route element={<DepotDetailPage />} path="firmware/:depot" />
        <Route element={<ChannelDetailPage />} path="firmware/:depot/:channel" />
        <Route element={<MemoryPage />} path="memory" />
      </Route>
      <Route element={<Navigate replace to="/overview" />} path="*" />
    </Routes>
  );
}
