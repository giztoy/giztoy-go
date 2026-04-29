import type { JSX, MutableRefObject } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { Info, MessageSquare, Mic, MicOff, Moon, Phone, PhoneOff, RadioTower, Sun, Video, VideoOff, X } from "lucide-react";

import { NoticeBanner } from "../../packages/components/banners";
import { Button } from "../../packages/components/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../packages/components/card";
import { cn } from "../../packages/components/utils";

interface SignalDescription {
  sdp: string;
  type: RTCSdpType;
}

interface NoticeState {
  message: string;
  tone: "error" | "success";
}

type CallState = "idle" | "starting" | "connecting" | "connected" | "closed" | "failed";
type ThemeMode = "dark" | "light";

const rpcPingMethod = "peer.ping";
const rpcChannelTimeoutMs = 10_000;

interface RPCLogEntry {
  elapsedMs?: number;
  error?: string;
  id: string;
  label: string;
  method: string;
  request: unknown;
  response?: unknown;
  status: "pending" | "ok" | "timeout" | "error";
}

interface NerdStats {
  downlinkKbps: number;
  inboundBytes: number;
  outboundBytes: number;
  packetsLost: number;
  rttMs?: number;
  uplinkKbps: number;
}

function App(): JSX.Element {
  const [callState, setCallState] = useState<CallState>("idle");
  const [connectionState, setConnectionState] = useState<RTCPeerConnectionState>("new");
  const [notice, setNotice] = useState<NoticeState | null>(null);
  const [rpcLog, setRPCLog] = useState<RPCLogEntry[]>([]);
  const [activeRPCChannels, setActiveRPCChannels] = useState<number>(0);
  const [cameraEnabled, setCameraEnabled] = useState<boolean>(true);
  const [micEnabled, setMicEnabled] = useState<boolean>(true);
  const [videoControlsVisible, setVideoControlsVisible] = useState<boolean>(true);
  const [statsOpen, setStatsOpen] = useState<boolean>(false);
  const [logDrawerOpen, setLogDrawerOpen] = useState<boolean>(false);
  const [theme, setTheme] = useState<ThemeMode>("dark");
  const [nerdStats, setNerdStats] = useState<NerdStats>({
    downlinkKbps: 0,
    inboundBytes: 0,
    outboundBytes: 0,
    packetsLost: 0,
    uplinkKbps: 0,
  });
  const [expandedRPCLogID, setExpandedRPCLogID] = useState<string | null>(null);

  const localVideoRef = useRef<HTMLVideoElement | null>(null);
  const remoteVideoRef = useRef<HTMLVideoElement | null>(null);
  const peerRef = useRef<RTCPeerConnection | null>(null);
  const localStreamRef = useRef<MediaStream | null>(null);
  const remoteStreamRef = useRef<MediaStream | null>(null);
  const hideVideoControlsTimerRef = useRef<number | null>(null);
  const lastStatsSampleRef = useRef<{ bytesReceived: number; bytesSent: number; timestamp: number } | null>(null);
  const isStarting = callState === "starting" || callState === "connecting";
  const isLive = callState === "connected" || connectionState === "connecting" || connectionState === "connected";
  const isPreCall = !isStarting && !isLive;
  const isLight = theme === "light";

  const clearVideoControlsTimer = useCallback(() => {
    if (hideVideoControlsTimerRef.current !== null) {
      window.clearTimeout(hideVideoControlsTimerRef.current);
      hideVideoControlsTimerRef.current = null;
    }
  }, []);

  const showVideoControls = useCallback(() => {
    setVideoControlsVisible(true);
    clearVideoControlsTimer();
    hideVideoControlsTimerRef.current = window.setTimeout(() => {
      setVideoControlsVisible(false);
      hideVideoControlsTimerRef.current = null;
    }, 2200);
  }, [clearVideoControlsTimer]);

  useEffect(() => clearVideoControlsTimer, [clearVideoControlsTimer]);

  const stopCall = useCallback(() => {
    peerRef.current?.close();
    peerRef.current = null;
    localStreamRef.current?.getTracks().forEach((track) => {
      track.stop();
    });
    localStreamRef.current = null;
    remoteStreamRef.current = null;
    if (localVideoRef.current !== null) {
      localVideoRef.current.srcObject = null;
    }
    if (remoteVideoRef.current !== null) {
      remoteVideoRef.current.srcObject = null;
    }
    setConnectionState("closed");
    setActiveRPCChannels(0);
    setCameraEnabled(true);
    setMicEnabled(true);
    setVideoControlsVisible(true);
    setStatsOpen(false);
    setLogDrawerOpen(false);
    setCallState("closed");
  }, []);

  useEffect(() => {
    if (isPreCall) {
      lastStatsSampleRef.current = null;
      return;
    }
    const interval = window.setInterval(() => {
      const peer = peerRef.current;
      if (peer === null) {
        return;
      }
      void samplePeerStats(peer, lastStatsSampleRef, setNerdStats);
    }, 1000);
    return () => {
      window.clearInterval(interval);
    };
  }, [isPreCall]);

  const startCall = useCallback(async () => {
    stopCall();
    setCallState("starting");
    setNotice(null);
    setRPCLog([]);
    showVideoControls();

    try {
      const localStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
        },
        video: true,
      });
      localStreamRef.current = localStream;
      setCameraEnabled(true);
      setMicEnabled(true);
      if (localVideoRef.current !== null) {
        localVideoRef.current.srcObject = localStream;
      }

      const remoteStream = new MediaStream();
      remoteStreamRef.current = remoteStream;
      if (remoteVideoRef.current !== null) {
        remoteVideoRef.current.srcObject = remoteStream;
      }

      const peer = new RTCPeerConnection();
      peerRef.current = peer;
      peer.onconnectionstatechange = () => {
        if (peerRef.current !== peer) {
          return;
        }
        setConnectionState(peer.connectionState);
        if (peer.connectionState === "connected") {
          setCallState("connected");
        }
        if (peer.connectionState === "failed") {
          setCallState("failed");
        }
        if (peer.connectionState === "closed") {
          setCallState("closed");
        }
      };
      peer.ontrack = (event) => {
        if (peerRef.current !== peer) {
          return;
        }
        for (const track of event.streams[0]?.getTracks() ?? [event.track]) {
          remoteStream.addTrack(track);
        }
      };

      const bootstrapChannel = peer.createDataChannel("rpc-bootstrap");
      bootstrapChannel.onopen = () => {
        bootstrapChannel.close();
      };

      for (const track of localStream.getTracks()) {
        peer.addTrack(track, localStream);
      }

      setCallState("connecting");
      const offer = await peer.createOffer();
      await peer.setLocalDescription(offer);
      await waitForIceGathering(peer);

      const localDescription = peer.localDescription;
      if (localDescription === null) {
        throw new Error("missing local WebRTC offer");
      }

      const response = await fetch("/webrtc/offer", {
        body: JSON.stringify({
          sdp: localDescription.sdp,
          type: localDescription.type,
        } satisfies SignalDescription),
        headers: {
          "Content-Type": "application/json",
        },
        method: "POST",
      });
      if (!response.ok) {
        const detail = await response.text();
        throw new Error(
          `signaling failed: ${response.status} ${response.statusText}${detail === "" ? "" : `: ${detail.trim()}`}`,
        );
      }
      const answer = (await response.json()) as SignalDescription;
      await peer.setRemoteDescription(answer);
      setNotice(null);
    } catch (error) {
      stopCall();
      setCallState("failed");
      setNotice({ message: toMessage(error), tone: "error" });
    }
  }, [showVideoControls, stopCall]);

  const sendPing = useCallback(() => {
    const peer = peerRef.current;
    if (peer === null || (peer.connectionState !== "connected" && peer.connectionState !== "connecting")) {
      setNotice({ message: "WebRTC connection is not ready for RPC yet.", tone: "error" });
      return;
    }
    const startedAt = performance.now();
    const request = {
      id: `play-${Date.now()}`,
      method: rpcPingMethod,
      params: {
        clientSendTime: Date.now(),
      },
      v: 1,
    };
    const label = `rpc:${request.id}`;
    const dataChannel = peer.createDataChannel(label);

    setActiveRPCChannels((count) => count + 1);
    setRPCLog((entries) => [
      {
        id: request.id,
        label,
        method: request.method,
        request,
        status: "pending",
      },
      ...entries,
    ]);

    let finished = false;
    const finish = (patch: Pick<RPCLogEntry, "status"> & Partial<RPCLogEntry>): void => {
      if (finished) {
        return;
      }
      finished = true;
      window.clearTimeout(timeout);
      setActiveRPCChannels((count) => Math.max(0, count - 1));
      setRPCLog((entries) =>
        entries.map((entry) =>
          entry.id === request.id
            ? {
                ...entry,
                elapsedMs: Math.round(performance.now() - startedAt),
                ...patch,
              }
            : entry,
        ),
      );
      safeCloseDataChannel(dataChannel);
    };
    const timeout = window.setTimeout(() => {
      finish({ error: `timed out after ${rpcChannelTimeoutMs}ms`, status: "timeout" });
    }, rpcChannelTimeoutMs);

    dataChannel.onopen = () => {
      try {
        dataChannel.send(JSON.stringify(request));
      } catch (error) {
        finish({ error: toMessage(error), status: "error" });
      }
    };
    dataChannel.onmessage = (event) => {
      try {
        const response = typeof event.data === "string" ? JSON.parse(event.data) : "[binary rpc response]";
        finish({ response, status: "ok" });
      } catch (error) {
        finish({ error: toMessage(error), status: "error" });
      }
    };
    dataChannel.onerror = () => {
      finish({ error: "data channel error", status: "error" });
    };
    dataChannel.onclose = () => {
      if (!finished) {
        finish({ error: "data channel closed before response", status: "error" });
      }
    };
  }, []);

  const toggleMic = useCallback(() => {
    const next = !micEnabled;
    localStreamRef.current?.getAudioTracks().forEach((track) => {
      track.enabled = next;
    });
    setMicEnabled(next);
  }, [micEnabled]);

  const toggleCamera = useCallback(() => {
    const next = !cameraEnabled;
    localStreamRef.current?.getVideoTracks().forEach((track) => {
      track.enabled = next;
    });
    setCameraEnabled(next);
  }, [cameraEnabled]);

  const rpcSucceeded = rpcLog.filter((entry) => entry.status === "ok").length;
  const rpcFailed = rpcLog.filter((entry) => entry.status === "error" || entry.status === "timeout").length;
  const actionButtonClass = (active: boolean): string =>
    cn(
      "rounded-full border px-3 transition",
      isLight
        ? "border-slate-200 bg-white text-slate-700 shadow-sm hover:bg-slate-100"
        : "border-white/10 bg-black/30 text-slate-200 hover:bg-white/10",
      active && (isLight ? "bg-slate-950 text-white hover:bg-slate-800" : "bg-white text-slate-950 hover:bg-slate-200"),
    );

  return (
    <main className={cn("min-h-screen transition-colors", isLight ? "bg-slate-100 text-slate-950" : "bg-slate-950 text-slate-50")}>
      {isPreCall ? (
        <DialScreen notice={notice} onStart={() => void startCall()} onThemeChange={setTheme} theme={theme} />
      ) : (
        <div className="mx-auto flex min-h-screen w-full max-w-7xl flex-col gap-4 p-4 lg:p-6">
          <header
            className={cn(
              "flex flex-wrap items-center justify-between gap-4 rounded-3xl border px-4 py-3 shadow-2xl transition-colors",
              isLight ? "border-slate-200 bg-white shadow-slate-300/30" : "border-white/10 bg-white/[0.05] shadow-black/20",
            )}
          >
            <div>
              <div className="text-xl font-semibold tracking-tight">GizClaw Play</div>
              <div className={cn("text-sm", isLight ? "text-slate-500" : "text-slate-400")}>
                WebRTC call surface with per-RPC DataChannels
              </div>
            </div>
            <div className="flex flex-wrap items-center justify-end gap-2">
              <ThemeSelector onThemeChange={setTheme} theme={theme} />
              <Button
                aria-label="Toggle call stats"
                className={actionButtonClass(statsOpen)}
                onClick={() => setStatsOpen((open) => !open)}
                type="button"
                variant="ghost"
              >
                <Info className="size-4" />
                Info
              </Button>
              <Button
                aria-label="Open RPC logs"
                className={actionButtonClass(logDrawerOpen)}
                onClick={() => setLogDrawerOpen((open) => !open)}
                type="button"
                variant="ghost"
              >
                <MessageSquare className="size-4" />
                Logs
                <span className="rounded-full bg-white/10 px-2 py-0.5 text-xs">{rpcLog.length}</span>
              </Button>
            </div>
          </header>

          <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(0,1fr)_22rem]">
            <section className="flex min-h-0 flex-col gap-4">
              <div
                className={cn(
                  "relative aspect-video w-full overflow-hidden rounded-3xl border bg-black shadow-2xl",
                  isLight ? "border-slate-200 shadow-slate-300/60" : "border-white/25 shadow-black/50 ring-1 ring-white/10",
                )}
                onFocus={showVideoControls}
                onMouseEnter={showVideoControls}
                onMouseLeave={() => {
                  clearVideoControlsTimer();
                  setVideoControlsVisible(false);
                }}
                onMouseMove={showVideoControls}
              >
                <video autoPlay className="h-full w-full bg-black object-cover" playsInline ref={remoteVideoRef} />

                {statsOpen ? (
                  <div className="absolute left-4 top-4">
                    <NerdStatsPanel
                      activeRPCChannels={activeRPCChannels}
                      callState={callState}
                      connectionState={connectionState}
                      rpcFailed={rpcFailed}
                      rpcMessages={rpcLog.length}
                      rpcSucceeded={rpcSucceeded}
                      stats={nerdStats}
                    />
                  </div>
                ) : null}

                <div className="absolute right-4 top-16 w-28 overflow-hidden rounded-2xl border border-white/15 bg-black shadow-2xl shadow-black/50 sm:w-36">
                  <div className="aspect-square">
                    <video
                      autoPlay
                      className={cn("h-full w-full bg-black object-cover", !cameraEnabled && "opacity-20")}
                      muted
                      playsInline
                      ref={localVideoRef}
                    />
                  </div>
                  <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 to-transparent px-3 py-2 text-xs text-slate-200">
                    You
                  </div>
                </div>

                <div
                  className={cn(
                    "absolute inset-x-0 bottom-0 flex items-center justify-center gap-3 bg-gradient-to-t from-black/85 to-transparent p-6",
                    "transition-opacity duration-300",
                    videoControlsVisible ? "opacity-100" : "pointer-events-none opacity-0",
                  )}
                >
                  <CallControl
                    active={micEnabled}
                    activeIcon={<Mic className="size-5" />}
                    inactiveIcon={<MicOff className="size-5" />}
                    label={micEnabled ? "Mute microphone" : "Unmute microphone"}
                    onClick={toggleMic}
                  />
                  <Button
                    aria-label="Hang Up"
                    className="size-14 rounded-full bg-red-600 text-white shadow-lg shadow-red-950/40 hover:bg-red-500"
                    onClick={stopCall}
                    type="button"
                  >
                    <PhoneOff className="size-5" />
                  </Button>
                  <CallControl
                    active={cameraEnabled}
                    activeIcon={<Video className="size-5" />}
                    inactiveIcon={<VideoOff className="size-5" />}
                    label={cameraEnabled ? "Turn camera off" : "Turn camera on"}
                    onClick={toggleCamera}
                  />
                </div>
              </div>

            </section>

            <aside className="flex min-h-0">
              <OperationsPanel
                className="min-h-full w-full"
                onSendPing={sendPing}
                theme={theme}
              />
            </aside>
          </div>
          <RPCLogDrawer
            expandedID={expandedRPCLogID}
            onClose={() => setLogDrawerOpen(false)}
            onToggleExpanded={setExpandedRPCLogID}
            open={logDrawerOpen}
            rpcLog={rpcLog}
            theme={theme}
          />
        </div>
      )}
    </main>
  );
}

function DialScreen({
  notice,
  onStart,
  onThemeChange,
  theme,
}: {
  notice: NoticeState | null;
  onStart: () => void;
  onThemeChange: (theme: ThemeMode) => void;
  theme: ThemeMode;
}): JSX.Element {
  return (
    <div className="relative flex min-h-screen flex-col items-center justify-center gap-6 p-6">
      <div className="absolute right-6 top-6">
        <ThemeSelector onThemeChange={onThemeChange} theme={theme} />
      </div>
      {notice ? <NoticeBanner className="max-w-xl" message={notice.message} tone={notice.tone} /> : null}
      <Button
        aria-label="Start Video Call"
        className={cn(
          "size-28 rounded-full bg-emerald-500 text-white shadow-2xl shadow-emerald-950/50",
          "hover:bg-emerald-400 focus-visible:ring-emerald-300",
        )}
        onClick={onStart}
        type="button"
      >
        <Phone className="size-10" />
        <span className="sr-only">Start Video Call</span>
      </Button>
    </div>
  );
}

function ThemeSelector({
  onThemeChange,
  theme,
}: {
  onThemeChange: (theme: ThemeMode) => void;
  theme: ThemeMode;
}): JSX.Element {
  const isLight = theme === "light";
  return (
    <div
      aria-label="Theme selector"
      className={cn(
        "flex rounded-full border p-1 text-sm",
        isLight ? "border-slate-200 bg-white shadow-sm" : "border-white/10 bg-black/30",
      )}
    >
      <button
        className={cn(
          "flex items-center gap-1 rounded-full px-3 py-1.5 transition",
          theme === "dark" ? "bg-slate-950 text-white" : "text-slate-600 hover:bg-slate-100",
        )}
        onClick={() => onThemeChange("dark")}
        type="button"
      >
        <Moon className="size-4" />
        Dark
      </button>
      <button
        className={cn(
          "flex items-center gap-1 rounded-full px-3 py-1.5 transition",
          theme === "light" ? "bg-slate-950 text-white" : "text-slate-300 hover:bg-white/10",
        )}
        onClick={() => onThemeChange("light")}
        type="button"
      >
        <Sun className="size-4" />
        Light
      </button>
    </div>
  );
}

function CallControl({
  active,
  activeIcon,
  inactiveIcon,
  label,
  onClick,
}: {
  active: boolean;
  activeIcon: JSX.Element;
  inactiveIcon: JSX.Element;
  label: string;
  onClick: () => void;
}): JSX.Element {
  return (
    <Button
      aria-label={label}
      className={cn(
        "size-12 rounded-full border border-white/10 bg-white/15 text-white backdrop-blur hover:bg-white/25",
        !active && "bg-white text-slate-950 hover:bg-slate-200",
      )}
      onClick={onClick}
      type="button"
      variant="ghost"
    >
      {active ? activeIcon : inactiveIcon}
    </Button>
  );
}

function StatusRow({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <div className="flex items-center justify-between gap-3 rounded-xl border border-white/10 bg-black/25 px-3 py-2">
      <span className="text-slate-400">{label}</span>
      <span className="font-medium text-slate-100">{value}</span>
    </div>
  );
}

function OperationsPanel({
  className,
  onSendPing,
  theme,
}: {
  className?: string;
  onSendPing: () => void;
  theme: ThemeMode;
}): JSX.Element {
  const isLight = theme === "light";
  return (
    <Card
      className={cn(
        isLight ? "border-slate-200 bg-white text-slate-950 shadow-xl shadow-slate-300/30" : "border-white/10 bg-white/[0.06] text-slate-50",
        className,
      )}
    >
      <CardHeader>
        <div className="flex items-center gap-2">
          <div className="flex size-9 items-center justify-center rounded-lg bg-primary/20 text-primary">
            <RadioTower className="size-4" />
          </div>
          <div>
            <CardTitle className="text-base">Operations</CardTitle>
            <CardDescription className={cn(isLight ? "text-slate-500" : "text-slate-300")}>
              Temporary controls before this becomes a menu.
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <Button className="w-full" onClick={onSendPing} type="button">
          <RadioTower className="size-4" />
          Send RPC Ping
        </Button>
      </CardContent>
    </Card>
  );
}

function RPCLogTable({
  className,
  expandedID,
  onClose,
  onToggleExpanded,
  rpcLog,
  theme,
}: {
  className?: string;
  expandedID: string | null;
  onClose?: () => void;
  onToggleExpanded: (id: string | null) => void;
  rpcLog: RPCLogEntry[];
  theme: ThemeMode;
}): JSX.Element {
  const isLight = theme === "light";
  return (
    <div
      className={cn(
        "min-h-0 overflow-auto rounded-xl border text-xs shadow-2xl",
        isLight ? "border-slate-200 bg-white text-slate-700" : "border-white/10 bg-slate-950 text-slate-200",
        className,
      )}
    >
      <div className="min-w-[48rem]">
        <div
          className={cn(
            "grid grid-cols-[5rem_9rem_7rem_5rem_minmax(0,1fr)_2.5rem] items-center gap-2 border-b px-3 py-2 text-[0.65rem] uppercase tracking-wide",
            isLight ? "border-slate-200 text-slate-500" : "border-white/10 text-slate-500",
          )}
        >
          <span className="text-center">Status</span>
          <span className="text-center">ID</span>
          <span className="text-center">Event</span>
          <span className="text-center">Cost</span>
          <span>Detail</span>
          {onClose === undefined ? (
            <span />
          ) : (
            <button
              aria-label="Close RPC logs"
              className={cn(
                "flex size-7 items-center justify-center rounded-full border transition",
                isLight
                  ? "border-slate-200 bg-white text-slate-700 hover:bg-slate-100"
                  : "border-white/10 bg-black/30 text-slate-200 hover:bg-white/10",
              )}
              onClick={onClose}
              type="button"
            >
              <X className="size-4" />
            </button>
          )}
        </div>
        {rpcLog.length === 0 ? (
          <div className="p-4 text-slate-500">No RPC calls yet. Each call opens a temporary DataChannel.</div>
        ) : (
          <div className={cn("divide-y", isLight ? "divide-slate-200" : "divide-white/10")}>
            {rpcLog.map((entry) => (
              <RPCLogRow
                entry={entry}
                expanded={expandedID === entry.id}
                key={entry.id}
                onToggle={() => onToggleExpanded(expandedID === entry.id ? null : entry.id)}
                theme={theme}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function RPCLogDrawer({
  expandedID,
  onClose,
  onToggleExpanded,
  open,
  rpcLog,
  theme,
}: {
  expandedID: string | null;
  onClose: () => void;
  onToggleExpanded: (id: string | null) => void;
  open: boolean;
  rpcLog: RPCLogEntry[];
  theme: ThemeMode;
}): JSX.Element | null {
  if (!open) {
    return null;
  }

  const isLight = theme === "light";

  return (
    <div className="fixed bottom-4 left-1/2 z-50 w-[min(96rem,calc(100vw-2rem))] -translate-x-1/2">
      <RPCLogTable
        className={cn("h-[min(66vh,44rem)]", isLight ? "shadow-slate-400/30" : "shadow-black/70")}
        expandedID={expandedID}
        onClose={onClose}
        onToggleExpanded={onToggleExpanded}
        rpcLog={rpcLog}
        theme={theme}
      />
    </div>
  );
}

function RPCLogRow({
  entry,
  expanded,
  onToggle,
  theme,
}: {
  entry: RPCLogEntry;
  expanded: boolean;
  onToggle: () => void;
  theme: ThemeMode;
}): JSX.Element {
  const isLight = theme === "light";
  return (
    <div className={cn("transition", isLight ? "hover:bg-slate-100" : "hover:bg-white/[0.04]")}>
      <button className="block w-full px-3 py-2 text-left" onClick={onToggle} type="button">
        <div className="grid grid-cols-[5rem_9rem_7rem_5rem_minmax(0,1fr)_2.5rem] items-center gap-2">
          <span className={cn("rounded-full px-2 py-0.5 text-center text-[0.65rem] font-semibold", rpcStatusClass(entry.status))}>
            {entry.status}
          </span>
          <span className={cn("truncate text-center font-mono", isLight ? "text-slate-700" : "text-slate-300")} title={entry.id}>
            {shortRPCID(entry.id)}
          </span>
          <span className={cn("truncate text-center font-mono", isLight ? "text-slate-600" : "text-slate-400")} title={entry.method}>
            {entry.method}
          </span>
          <span className="text-center font-mono text-slate-500">{entry.elapsedMs === undefined ? "-" : `${entry.elapsedMs}ms`}</span>
          <span className={cn("truncate font-mono", isLight ? "text-slate-500" : "text-slate-400")} title={rpcLogSummary(entry)}>
            {rpcLogSummary(entry)}
          </span>
          <span aria-hidden="true" />
        </div>
      </button>
      {expanded ? (
        <div
          className={cn(
            "mx-3 mb-3 space-y-1 rounded-lg border p-3 font-mono text-[0.7rem]",
            isLight ? "border-slate-200 bg-white" : "border-white/10 bg-black/30",
          )}
        >
          <div className="text-slate-500">request</div>
          <pre className={cn("whitespace-pre-wrap break-words", isLight ? "text-slate-700" : "text-slate-300")}>
            {JSON.stringify(entry.request, null, 2)}
          </pre>
          {entry.response === undefined ? null : (
            <>
              <div className="pt-2 text-slate-500">response</div>
              <pre className="whitespace-pre-wrap break-words text-emerald-300">{JSON.stringify(entry.response, null, 2)}</pre>
            </>
          )}
          {entry.error === undefined ? null : <div className="pt-2 text-red-300">error {entry.error}</div>}
        </div>
      ) : null}
    </div>
  );
}

function shortRPCID(id: string): string {
  if (id.length <= 10) {
    return id;
  }
  return id.slice(-10);
}

function rpcLogSummary(entry: RPCLogEntry): string {
  if (entry.error !== undefined) {
    return entry.error;
  }
  if (entry.response !== undefined) {
    return JSON.stringify(entry.response);
  }
  return entry.status === "pending" ? "waiting for response" : "-";
}

function NerdStatsPanel({
  activeRPCChannels,
  callState,
  connectionState,
  rpcFailed,
  rpcMessages,
  rpcSucceeded,
  stats,
}: {
  activeRPCChannels: number;
  callState: CallState;
  connectionState: RTCPeerConnectionState;
  rpcFailed: number;
  rpcMessages: number;
  rpcSucceeded: number;
  stats: NerdStats;
}): JSX.Element {
  return (
    <div className="w-72 rounded-2xl border border-white/15 bg-black/70 p-3 text-xs text-slate-200 shadow-2xl shadow-black/50 backdrop-blur">
      <div className="mb-2 font-semibold text-slate-100">Stats for nerds</div>
      <div className="space-y-1">
        <StatusRow label="Call" value={callState} />
        <StatusRow label="Peer" value={connectionState} />
        <StatusRow label="Downlink" value={`${stats.downlinkKbps.toFixed(1)} kbps`} />
        <StatusRow label="Uplink" value={`${stats.uplinkKbps.toFixed(1)} kbps`} />
        <StatusRow label="RTT" value={stats.rttMs === undefined ? "-" : `${stats.rttMs.toFixed(0)} ms`} />
        <StatusRow label="Packets lost" value={String(stats.packetsLost)} />
        <StatusRow label="RPC total" value={String(rpcMessages)} />
        <StatusRow label="RPC ok/fail" value={`${rpcSucceeded}/${rpcFailed}`} />
        <StatusRow label="RPC active" value={String(activeRPCChannels)} />
      </div>
    </div>
  );
}

function rpcStatusClass(status: RPCLogEntry["status"]): string {
  switch (status) {
    case "ok":
      return "bg-emerald-400/15 text-emerald-300";
    case "pending":
      return "bg-sky-400/15 text-sky-300";
    case "timeout":
      return "bg-amber-400/15 text-amber-300";
    case "error":
      return "bg-red-400/15 text-red-300";
  }
}

function safeCloseDataChannel(dataChannel: RTCDataChannel): void {
  if (dataChannel.readyState === "closed" || dataChannel.readyState === "closing") {
    return;
  }
  dataChannel.close();
}

async function samplePeerStats(
  peer: RTCPeerConnection,
  lastSampleRef: MutableRefObject<{ bytesReceived: number; bytesSent: number; timestamp: number } | null>,
  setStats: (stats: NerdStats) => void,
): Promise<void> {
  const report = await peer.getStats();
  let bytesReceived = 0;
  let bytesSent = 0;
  let packetsLost = 0;
  let rttMs: number | undefined;

  report.forEach((raw) => {
    const item = raw as RTCStats & {
      bytesReceived?: number;
      bytesSent?: number;
      currentRoundTripTime?: number;
      packetsLost?: number;
      selected?: boolean;
      state?: string;
    };
    if (item.type === "inbound-rtp") {
      bytesReceived += item.bytesReceived ?? 0;
      packetsLost += item.packetsLost ?? 0;
    }
    if (item.type === "outbound-rtp") {
      bytesSent += item.bytesSent ?? 0;
    }
    if (item.type === "candidate-pair" && (item.selected === true || item.state === "succeeded")) {
      if (typeof item.currentRoundTripTime === "number") {
        rttMs = item.currentRoundTripTime * 1000;
      }
    }
  });

  const now = performance.now();
  const previous = lastSampleRef.current;
  let downlinkKbps = 0;
  let uplinkKbps = 0;
  if (previous !== null && now > previous.timestamp) {
    const seconds = (now - previous.timestamp) / 1000;
    downlinkKbps = ((bytesReceived - previous.bytesReceived) * 8) / seconds / 1000;
    uplinkKbps = ((bytesSent - previous.bytesSent) * 8) / seconds / 1000;
  }
  lastSampleRef.current = { bytesReceived, bytesSent, timestamp: now };
  setStats({
    downlinkKbps: Math.max(0, downlinkKbps),
    inboundBytes: bytesReceived,
    outboundBytes: bytesSent,
    packetsLost,
    rttMs,
    uplinkKbps: Math.max(0, uplinkKbps),
  });
}

async function waitForIceGathering(peer: RTCPeerConnection): Promise<void> {
  if (peer.iceGatheringState === "complete") {
    return;
  }
  await new Promise<void>((resolve) => {
    const handleStateChange = (): void => {
      if (peer.iceGatheringState === "complete") {
        peer.removeEventListener("icegatheringstatechange", handleStateChange);
        resolve();
      }
    };
    peer.addEventListener("icegatheringstatechange", handleStateChange);
  });
}

function toMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

const root = document.querySelector<HTMLElement>("#app");

if (root === null) {
  throw new Error("missing #app root");
}

createRoot(root).render(<App />);
