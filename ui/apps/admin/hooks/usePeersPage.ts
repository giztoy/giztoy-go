import { useCallback, useEffect, useMemo, useState } from "react";

import { expectData, toMessage } from "../../../packages/components/api";
import { listDepots, listGears } from "../../../packages/adminservice";
import { getServerInfo, type ServerInfo } from "../../../packages/serverpublic";

import type { Depot, Registration } from "../../../packages/adminservice";

export const PEER_PAGE_LIMIT = 50;

export interface PeerListState {
  cursor: string | null;
  hasNext: boolean;
  history: Array<string | null>;
  nextCursor: string | null;
}

export interface PeersPageState {
  depots: Depot[];
  error: string;
  gears: Registration[];
  loading: boolean;
  serverInfo: ServerInfo | null;
}

export function usePeersPage(): {
  dashboard: PeersPageState;
  peerList: PeerListState;
  peerPageNumber: number;
  filter: string;
  filteredGears: Registration[];
  loadDashboard: (cursor: string | null, history: Array<string | null>) => Promise<void>;
  nextPage: () => void;
  prevPage: () => void;
  refreshDashboard: () => Promise<void>;
  setFilter: (value: string) => void;
} {
  const [filter, setFilter] = useState("");
  const [dashboard, setDashboard] = useState<PeersPageState>({
    depots: [],
    error: "",
    gears: [],
    loading: true,
    serverInfo: null,
  });
  const [peerList, setPeerList] = useState<PeerListState>({
    cursor: null,
    hasNext: false,
    history: [],
    nextCursor: null,
  });

  const loadDashboard = useCallback(async (cursor: string | null, history: Array<string | null>) => {
    setDashboard((current) => ({ ...current, error: "", loading: true }));
    try {
      const [serverInfo, registrations, depots] = await Promise.all([
        expectData(getServerInfo()),
        expectData(
          listGears({
            query: {
              cursor: cursor ?? undefined,
              limit: PEER_PAGE_LIMIT,
            },
          }),
        ),
        expectData(listDepots()),
      ]);

      setDashboard({
        depots: depots.items ?? [],
        error: "",
        gears: registrations.items ?? [],
        loading: false,
        serverInfo,
      });
      setPeerList({
        cursor,
        hasNext: registrations.has_next,
        history,
        nextCursor: registrations.next_cursor ?? null,
      });
    } catch (error) {
      setDashboard((current) => ({
        ...current,
        error: toMessage(error),
        loading: false,
      }));
    }
  }, []);

  const refreshDashboard = useCallback(async () => {
    await loadDashboard(peerList.cursor, peerList.history);
  }, [peerList.cursor, peerList.history, loadDashboard]);

  useEffect(() => {
    void loadDashboard(null, []);
  }, [loadDashboard]);

  const filteredGears = useMemo(() => {
    if (filter.trim() === "") {
      return dashboard.gears;
    }
    const query = filter.trim().toLowerCase();
    return dashboard.gears.filter((gear) =>
      [gear.public_key, gear.role, gear.status, gear.auto_registered ? "auto" : "manual"].some((value) =>
        value.toLowerCase().includes(query),
      ),
    );
  }, [dashboard.gears, filter]);

  const nextPage = useCallback(() => {
    if (peerList.nextCursor === null) {
      return;
    }
    void loadDashboard(peerList.nextCursor, [...peerList.history, peerList.cursor]);
  }, [peerList.cursor, peerList.history, peerList.nextCursor, loadDashboard]);

  const prevPage = useCallback(() => {
    if (peerList.history.length === 0) {
      return;
    }
    const previousCursor = peerList.history[peerList.history.length - 1] ?? null;
    void loadDashboard(previousCursor, peerList.history.slice(0, -1));
  }, [peerList.history, loadDashboard]);

  const peerPageNumber = peerList.history.length + 1;

  return {
    dashboard,
    peerList,
    peerPageNumber,
    filter,
    filteredGears,
    loadDashboard,
    nextPage,
    prevPage,
    refreshDashboard,
    setFilter,
  };
}
