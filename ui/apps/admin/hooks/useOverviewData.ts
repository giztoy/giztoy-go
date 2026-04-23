import { useEffect, useState } from "react";

import { expectData, toMessage } from "../../../packages/components/api";
import { listDepots, listGears } from "../../../packages/adminservice";
import { getServerInfo, type ServerInfo } from "../../../packages/serverpublic";

import type { Depot, Registration } from "../../../packages/adminservice";

import { DEVICE_PAGE_LIMIT } from "./useDevicesPage";

export interface OverviewData {
  depots: Depot[];
  error: string;
  gears: Registration[];
  loading: boolean;
  serverInfo: ServerInfo | null;
}

export function useOverviewData(): OverviewData {
  const [data, setData] = useState<OverviewData>({
    depots: [],
    error: "",
    gears: [],
    loading: true,
    serverInfo: null,
  });

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const [serverInfo, registrations, depots] = await Promise.all([
          expectData(getServerInfo()),
          expectData(
            listGears({
              query: { limit: DEVICE_PAGE_LIMIT },
            }),
          ),
          expectData(listDepots()),
        ]);
        if (cancelled) {
          return;
        }
        setData({
          depots: depots.items ?? [],
          error: "",
          gears: registrations.items ?? [],
          loading: false,
          serverInfo,
        });
      } catch (error) {
        if (cancelled) {
          return;
        }
        setData((current) => ({
          ...current,
          error: toMessage(error),
          loading: false,
        }));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return data;
}
