import { useCallback, useEffect, useState } from "react";

import { expectData, toMessage } from "../../../packages/components/api";
import {
  getGear,
  getGearConfig,
  getGearInfo,
  getGearOta,
  getGearRuntime,
  type Configuration,
  type DeviceInfo,
  type Registration,
  type Runtime,
} from "../../../packages/adminservice";

export interface GearDetail {
  config: Configuration | null;
  info: DeviceInfo | null;
  ota: unknown | null;
  registration: Registration | null;
  runtime: Runtime | null;
}

export interface GearDetailState {
  data: GearDetail | null;
  error: string;
  loading: boolean;
}

export function usePeerDetail(publicKey: string | undefined): GearDetailState & { reload: () => Promise<void> } {
  const [state, setState] = useState<GearDetailState>({
    data: null,
    error: "",
    loading: false,
  });

  const load = useCallback(async () => {
    if (publicKey === undefined || publicKey === "") {
      setState({ data: null, error: "", loading: false });
      return;
    }

    setState({ data: null, error: "", loading: true });
    try {
      const registration = await expectData(getGear({ path: { publicKey } }));
      const [info, config, runtime, ota] = await Promise.all([
        loadOptional(() => expectData(getGearInfo({ path: { publicKey } }))),
        loadOptional(() => expectData(getGearConfig({ path: { publicKey } }))),
        loadOptional(() => expectData(getGearRuntime({ path: { publicKey } }))),
        loadOptional(() => expectData(getGearOta({ path: { publicKey } }))),
      ]);

      setState({
        data: {
          config,
          info,
          ota,
          registration,
          runtime,
        },
        error: "",
        loading: false,
      });
    } catch (error) {
      setState({
        data: null,
        error: toMessage(error),
        loading: false,
      });
    }
  }, [publicKey]);

  useEffect(() => {
    void load();
  }, [load]);

  return { ...state, reload: load };
}

async function loadOptional<T>(load: () => Promise<T>): Promise<T | null> {
  try {
    return await load();
  } catch {
    return null;
  }
}
