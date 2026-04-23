import { useCallback, useEffect, useState } from "react";

import { expectData, toMessage } from "../../../packages/components/api";
import { getChannel, type DepotRelease } from "../../../packages/adminservice";

export interface ChannelDetailState {
  data: DepotRelease | null;
  error: string;
  loading: boolean;
}

export function useChannelDetail(
  depotName: string | undefined,
  channelName: string | undefined,
): ChannelDetailState & { reload: () => Promise<void> } {
  const [state, setState] = useState<ChannelDetailState>({
    data: null,
    error: "",
    loading: false,
  });

  const load = useCallback(async () => {
    if (depotName === undefined || depotName === "" || channelName === undefined || channelName === "") {
      setState({ data: null, error: "", loading: false });
      return;
    }

    setState({ data: null, error: "", loading: true });
    try {
      const release = await expectData(
        getChannel({
          path: { channel: channelName, depot: depotName },
        }),
      );
      setState({ data: release, error: "", loading: false });
    } catch (error) {
      setState({
        data: null,
        error: toMessage(error),
        loading: false,
      });
    }
  }, [channelName, depotName]);

  useEffect(() => {
    void load();
  }, [load]);

  return { ...state, reload: load };
}
