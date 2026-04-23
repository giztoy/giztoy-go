import { useCallback, useEffect, useState } from "react";

import { expectData, toMessage } from "../../../packages/components/api";
import { getDepot, type Depot } from "../../../packages/adminservice";

export interface DepotDetailState {
  data: Depot | null;
  error: string;
  loading: boolean;
}

export function useDepotDetail(depotName: string | undefined): DepotDetailState & { reload: () => Promise<void> } {
  const [state, setState] = useState<DepotDetailState>({
    data: null,
    error: "",
    loading: false,
  });

  const load = useCallback(async () => {
    if (depotName === undefined || depotName === "") {
      setState({ data: null, error: "", loading: false });
      return;
    }

    setState({ data: null, error: "", loading: true });
    try {
      const depot = await expectData(getDepot({ path: { depot: depotName } }));
      setState({ data: depot, error: "", loading: false });
    } catch (error) {
      setState({
        data: null,
        error: toMessage(error),
        loading: false,
      });
    }
  }, [depotName]);

  useEffect(() => {
    void load();
  }, [load]);

  return { ...state, reload: load };
}
