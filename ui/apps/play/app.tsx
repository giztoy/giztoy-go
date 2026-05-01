import type { JSX } from "react";
import { useCallback, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { Activity, Moon, Phone, PhoneOff, RadioTower, Sun } from "lucide-react";

import { Button } from "../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../packages/components/card";
import { cn } from "../../packages/components/utils";

interface RpcRequest {
  id: string;
  method: string;
  params?: unknown;
}

interface RpcResponse {
  id: string;
  result?: unknown;
  error?: {
    message: string;
  };
}

interface RpcLogEntry {
  id: string;
  event: string;
  detail: string;
}

type CallStatus = "Idle" | "Starting" | "Connected" | "RPC failed" | "Ended";
type Theme = "dark" | "light";

function App(): JSX.Element {
  const [theme, setTheme] = useState<Theme>("dark");
  const [status, setStatus] = useState<CallStatus>("Idle");
  const [logs, setLogs] = useState<RpcLogEntry[]>([]);
  const [rpcSent, setRpcSent] = useState(0);
  const [rpcReceived, setRpcReceived] = useState(0);
  const uiPeerRef = useRef<RTCPeerConnection | null>(null);
  const backendPeerRef = useRef<RTCPeerConnection | null>(null);
  const dataChannelRef = useRef<RTCDataChannel | null>(null);

  const appendLog = useCallback((event: string, detail: unknown) => {
    setLogs((current) => [
      {
        id: `${Date.now()}-${current.length + 1}`,
        event,
        detail: typeof detail === "string" ? detail : JSON.stringify(detail),
      },
      ...current,
    ].slice(0, 100));
  }, []);

  const closeCall = useCallback(() => {
    dataChannelRef.current?.close();
    uiPeerRef.current?.close();
    backendPeerRef.current?.close();
    dataChannelRef.current = null;
    uiPeerRef.current = null;
    backendPeerRef.current = null;
    setStatus("Ended");
    appendLog("call.closed", "WebRTC call closed");
  }, [appendLog]);

  const handleRPC = useCallback(async (request: RpcRequest): Promise<RpcResponse> => {
    try {
      if (request.method !== "server.info.get") {
        throw new Error(`unsupported method ${request.method}`);
      }
      const response = await fetch("/api/public/server-info");
      if (!response.ok) {
        const message = (await response.text()).trim() || `${response.status} ${response.statusText}`;
        throw new Error(message);
      }
      return { id: request.id, result: await response.json() };
    } catch (error) {
      return {
        id: request.id,
        error: {
          message: error instanceof Error ? error.message : String(error),
        },
      };
    }
  }, []);

  const startCall = useCallback(async () => {
    if (status === "Starting") {
      return;
    }
    closeCall();
    setStatus("Starting");
    setLogs([]);
    setRpcSent(0);
    setRpcReceived(0);

    const uiPeer = new RTCPeerConnection({ iceServers: [] });
    const backendPeer = new RTCPeerConnection({ iceServers: [] });
    uiPeerRef.current = uiPeer;
    backendPeerRef.current = backendPeer;

    uiPeer.onicecandidate = (event) => {
      if (event.candidate) {
        void backendPeer.addIceCandidate(event.candidate);
      }
    };
    backendPeer.onicecandidate = (event) => {
      if (event.candidate) {
        void uiPeer.addIceCandidate(event.candidate);
      }
    };
    uiPeer.onconnectionstatechange = () => appendLog("webrtc.state", uiPeer.connectionState);

    backendPeer.ondatachannel = (event) => {
      const backendChannel = event.channel;
      appendLog("backend.datachannel", backendChannel.label);
      backendChannel.onmessage = (message) => {
        void (async () => {
          const request = JSON.parse(String(message.data)) as RpcRequest;
          appendLog("rpc.request", request.method);
          const response = await handleRPC(request);
          backendChannel.send(JSON.stringify(response));
        })();
      };
    };

    const rpcChannel = uiPeer.createDataChannel("rpc", { ordered: true });
    dataChannelRef.current = rpcChannel;
    rpcChannel.onopen = () => {
      setStatus("Connected");
      appendLog("rpc.open", "RPC data channel open");
      const request: RpcRequest = {
        id: crypto.randomUUID(),
        method: "server.info.get",
        params: {},
      };
      setRpcSent((current) => current + 1);
      appendLog("rpc.send", request.method);
      rpcChannel.send(JSON.stringify(request));
    };
    rpcChannel.onmessage = (message) => {
      const response = JSON.parse(String(message.data)) as RpcResponse;
      setRpcReceived((current) => current + 1);
      if (response.error) {
        setStatus("RPC failed");
        appendLog("rpc.error", response.error.message);
        return;
      }
      appendLog("rpc.response", response.result ?? {});
    };
    rpcChannel.onclose = () => appendLog("rpc.close", "RPC data channel closed");

    const offer = await uiPeer.createOffer();
    await uiPeer.setLocalDescription(offer);
    await backendPeer.setRemoteDescription(offer);
    const answer = await backendPeer.createAnswer();
    await backendPeer.setLocalDescription(answer);
    await uiPeer.setRemoteDescription(answer);
  }, [appendLog, closeCall, handleRPC, status]);

  const dark = theme === "dark";

  return (
    <main className={cn("min-h-screen transition-colors", dark ? "bg-slate-950 text-slate-50" : "bg-slate-50 text-slate-950")}>
      <div className="mx-auto grid min-h-screen w-full max-w-7xl gap-6 px-4 py-6 lg:grid-cols-[minmax(0,1.4fr)_24rem]">
        <section className="grid gap-6 lg:grid-rows-[minmax(0,1fr)_18rem]">
          <Card className={cn("overflow-hidden border", dark ? "border-slate-700 bg-slate-900" : "border-slate-200 bg-white")}>
            <CardContent className="relative flex aspect-square min-h-[30rem] items-center justify-center p-0">
              <div className="absolute left-4 top-4 rounded-lg bg-slate-950/70 px-3 py-2 text-xs text-slate-100">
                <div className="font-semibold">WebRTC Play</div>
                <div>Status: {status}</div>
                <div>RPC up/down: {rpcSent}/{rpcReceived}</div>
              </div>
              <div className={cn("flex size-40 items-center justify-center rounded-full border", dark ? "border-cyan-400/50 bg-cyan-400/10" : "border-cyan-600/40 bg-cyan-100")}>
                <RadioTower className={cn("size-16", status === "Connected" ? "text-cyan-400" : "text-slate-400")} />
              </div>
            </CardContent>
          </Card>

          <Card className={cn("border shadow-none", dark ? "border-slate-700 bg-slate-900" : "border-slate-200 bg-white")}>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">RPC Log</CardTitle>
              <CardDescription>One line per WebRTC data channel event. Details stay compact for the drawer-style layout.</CardDescription>
            </CardHeader>
            <CardContent>
              <div className={cn("h-48 overflow-auto rounded-lg border text-sm", dark ? "border-slate-700" : "border-slate-200")}>
                <table className="w-full table-fixed border-collapse">
                  <tbody>
                    {logs.length === 0 ? (
                      <tr>
                        <td className="px-3 py-3 text-slate-500">No RPC events yet.</td>
                      </tr>
                    ) : logs.map((entry) => (
                      <tr className={dark ? "border-b border-slate-800" : "border-b border-slate-100"} key={entry.id}>
                        <td className="w-36 px-3 py-2 font-medium">{entry.event}</td>
                        <td className="truncate px-3 py-2 text-slate-500">{entry.detail}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </section>

        <aside className="flex flex-col justify-center gap-4">
          <Card className={cn("border shadow-none", dark ? "border-slate-700 bg-slate-900" : "border-slate-200 bg-white")}>
            <CardHeader>
              <CardTitle>Controls</CardTitle>
              <CardDescription>Start the local WebRTC call and tunnel JSON RPC over the `rpc` data channel.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <Button className="w-full" disabled={status === "Starting"} onClick={() => void startCall()} type="button">
                <Phone className="size-4" />
                Start Video Call
              </Button>
              <Button className="w-full" onClick={closeCall} type="button" variant="outline">
                <PhoneOff className="size-4" />
                End Call
              </Button>
            </CardContent>
          </Card>

          <Card className={cn("border shadow-none", dark ? "border-slate-700 bg-slate-900" : "border-slate-200 bg-white")}>
            <CardHeader>
              <CardTitle>Theme</CardTitle>
              <CardDescription>Choose the play surface theme.</CardDescription>
            </CardHeader>
            <CardContent className="grid grid-cols-2 gap-2">
              <Button onClick={() => setTheme("dark")} type="button" variant={dark ? "default" : "outline"}>
                <Moon className="size-4" />
                Dark
              </Button>
              <Button onClick={() => setTheme("light")} type="button" variant={!dark ? "default" : "outline"}>
                <Sun className="size-4" />
                Light
              </Button>
            </CardContent>
          </Card>

          <Card className={cn("border shadow-none", dark ? "border-slate-700 bg-slate-900" : "border-slate-200 bg-white")}>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Activity className="size-4" />
                Stats
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <InfoRow label="WebRTC status" value={status} />
              <InfoRow label="Data channel" value={dataChannelRef.current?.readyState ?? "closed"} />
              <InfoRow label="RPC sent" value={String(rpcSent)} />
              <InfoRow label="RPC received" value={String(rpcReceived)} />
            </CardContent>
          </Card>
        </aside>
      </div>
    </main>
  );
}

function InfoRow({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <div className="flex items-start justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium text-foreground">{value}</span>
    </div>
  );
}

const root = document.querySelector<HTMLElement>("#app");

if (root === null) {
  throw new Error("missing #app root");
}

createRoot(root).render(<App />);
