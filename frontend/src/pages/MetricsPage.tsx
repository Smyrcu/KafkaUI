import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import * as chrono from "chrono-node";
import { api } from "@/lib/api";
import type { BrokerMetricsInfo, TimestampedMetrics } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorAlert } from "@/components/ErrorAlert";
import { EmptyState } from "@/components/EmptyState";
import { PageHeader } from "@/components/PageHeader";
import { TableSkeleton } from "@/components/PageSkeleton";
import { Input } from "@/components/ui/input";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { BarChart3, ArrowDownToLine, ArrowUpFromLine, Mail, AlertTriangle, Crown, WifiOff, Calendar, ChevronDown } from "lucide-react";
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from "recharts";

const PRESETS = [
  { key: "live", shorthand: "LIVE", label: "15 Minutes", duration: "15m", live: true },
  { key: "15m", shorthand: "15m", label: "Past 15 Minutes", duration: "15m" },
  { key: "1h", shorthand: "1h", label: "Past 1 Hour", duration: "1h" },
  { key: "4h", shorthand: "4h", label: "Past 4 Hours", duration: "4h" },
  { key: "1d", shorthand: "1d", label: "Past 1 Day", duration: "1d" },
  { key: "2d", shorthand: "2d", label: "Past 2 Days", duration: "2d" },
  { key: "3d", shorthand: "3d", label: "Past 3 Days", duration: "3d" },
  { key: "7d", shorthand: "7d", label: "Past 7 Days", duration: "7d" },
  { key: "15d", shorthand: "15d", label: "Past 15 Days", duration: "15d" },
  { key: "1mo", shorthand: "1mo", label: "Past 1 Month", duration: "30d" },
] as const;

const MORE_PRESETS = [
  { key: "45m", shorthand: "45m", label: "Past 45 Minutes", duration: "45m" },
  { key: "12h", shorthand: "12h", label: "Past 12 Hours", duration: "12h" },
  { key: "2w", shorthand: "2w", label: "Past 2 Weeks", duration: "14d" },
] as const;

function formatRate(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M/s`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K/s`;
  return `${value.toFixed(1)}/s`;
}

function formatBytes(value: number): string {
  if (value >= 1_073_741_824) return `${(value / 1_073_741_824).toFixed(1)} GB/s`;
  if (value >= 1_048_576) return `${(value / 1_048_576).toFixed(1)} MB/s`;
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KB/s`;
  return `${value.toFixed(0)} B/s`;
}

function formatChartBytes(value: number): string {
  if (value >= 1_048_576) return `${(value / 1_048_576).toFixed(0)} MB`;
  if (value >= 1024) return `${(value / 1024).toFixed(0)} KB`;
  return `${value.toFixed(0)} B`;
}

function formatTime(iso: string, range_: string): string {
  const d = new Date(iso);
  if (["3d", "7d", "14d"].includes(range_)) {
    return d.toLocaleDateString("en-GB", { day: "2-digit", month: "short" });
  }
  if (["1d", "6h", "12h", "24h"].includes(range_)) {
    return d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" });
  }
  return d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function effectiveRange(custom: { from: string; to: string } | null, preset: string): string {
  if (!custom) return preset;
  const ms = new Date(custom.to).getTime() - new Date(custom.from).getTime();
  const hours = ms / 3600000;
  if (hours >= 72) return "7d";
  if (hours >= 24) return "1d";
  if (hours >= 6) return "6h";
  return "1h";
}

function toChartData(history: TimestampedMetrics[], range_: string) {
  return history.map((p) => ({
    time: formatTime(p.time, range_),
    bytesIn: p.metrics.bytesInPerSec,
    bytesOut: p.metrics.bytesOutPerSec,
    messagesIn: p.metrics.messagesInPerSec,
  }));
}

function ThroughputChart({ history, range_ }: { history: TimestampedMetrics[]; range_: string }) {
  const data = toChartData(history, range_);
  if (data.length < 2) {
    return (
      <div className="flex items-center justify-center h-52 text-sm text-muted-foreground">
        Collecting data... ({data.length} point{data.length !== 1 ? "s" : ""}, need at least 2)
      </div>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={220}>
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" className="opacity-30" />
        <XAxis dataKey="time" tick={{ fontSize: 11 }} interval="preserveStartEnd" />
        <YAxis tickFormatter={formatChartBytes} tick={{ fontSize: 11 }} width={65} />
        <Tooltip
          formatter={((value: number | undefined, name?: string) => [
            formatBytes(value ?? 0),
            name === "bytesIn" ? "Bytes In" : "Bytes Out",
          ]) as any}
          contentStyle={{ fontSize: 12, backgroundColor: "hsl(var(--card))", border: "1px solid hsl(var(--border))", borderRadius: 6 }}
        />
        <Line type="monotone" dataKey="bytesIn" stroke="hsl(var(--primary))" strokeWidth={2} dot={false} name="bytesIn" />
        <Line type="monotone" dataKey="bytesOut" stroke="hsl(var(--success, 142 71% 45%))" strokeWidth={2} dot={false} name="bytesOut" />
      </LineChart>
    </ResponsiveContainer>
  );
}

function MessagesChart({ history, range_ }: { history: TimestampedMetrics[]; range_: string }) {
  const data = toChartData(history, range_);
  if (data.length < 2) return null;

  return (
    <ResponsiveContainer width="100%" height={220}>
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" className="opacity-30" />
        <XAxis dataKey="time" tick={{ fontSize: 11 }} interval="preserveStartEnd" />
        <YAxis tickFormatter={(v) => formatRate(v)} tick={{ fontSize: 11 }} width={65} />
        <Tooltip
          formatter={((value: number | undefined) => [formatRate(value ?? 0), "Messages In"]) as any}
          contentStyle={{ fontSize: 12, backgroundColor: "hsl(var(--card))", border: "1px solid hsl(var(--border))", borderRadius: 6 }}
        />
        <Line type="monotone" dataKey="messagesIn" stroke="hsl(var(--warning, 38 92% 50%))" strokeWidth={2} dot={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}

function BrokerSection({ broker, range_ }: { broker: BrokerMetricsInfo; range_: string }) {
  if (broker.error) {
    return (
      <Card className="animate-scale-in col-span-full">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-lg">Broker {broker.id}</CardTitle>
            <Badge variant="destructive">error</Badge>
          </div>
          <p className="text-xs text-muted-foreground">{broker.host}</p>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-destructive">{broker.error}</p>
        </CardContent>
      </Card>
    );
  }

  const m = broker.metrics!;
  const history = broker.history ?? [];
  const stats = [
    { label: "Bytes In", value: formatBytes(m.bytesInPerSec), icon: ArrowDownToLine, warn: false },
    { label: "Bytes Out", value: formatBytes(m.bytesOutPerSec), icon: ArrowUpFromLine, warn: false },
    { label: "Messages In", value: formatRate(m.messagesInPerSec), icon: Mail, warn: false },
    { label: "Under-replicated", value: String(m.underReplicatedPartitions), icon: AlertTriangle, warn: m.underReplicatedPartitions > 0 },
    { label: "Active Controller", value: String(m.activeControllerCount), icon: Crown, warn: false },
    { label: "Offline Partitions", value: String(m.offlinePartitionsCount), icon: WifiOff, warn: m.offlinePartitionsCount > 0 },
  ];

  return (
    <>
      <Card className="animate-scale-in">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-lg">Broker {broker.id}</CardTitle>
            <Badge variant="success">healthy</Badge>
          </div>
          <p className="text-xs text-muted-foreground">{broker.host}</p>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-3">
            {stats.map((s) => (
              <div key={s.label} className="flex items-center gap-2">
                <s.icon className={`h-4 w-4 ${s.warn ? "text-destructive" : "text-muted-foreground"}`} />
                <div>
                  <p className={`text-sm font-medium ${s.warn ? "text-destructive" : ""}`}>{s.value}</p>
                  <p className="text-xs text-muted-foreground">{s.label}</p>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card className="animate-scale-in lg:col-span-2">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">
            Throughput — Broker {broker.id}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ThroughputChart history={history} range_={range_} />
        </CardContent>
      </Card>

      <Card className="animate-scale-in lg:col-span-3">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-muted-foreground">
            Messages/s — Broker {broker.id}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <MessagesChart history={history} range_={range_} />
        </CardContent>
      </Card>
    </>
  );
}

function parseTimeInput(input: string): { from: Date; to: Date } | null {
  const trimmed = input.trim();
  if (!trimmed) return null;

  // Unix timestamp range: "1234567890 - 1234567899"
  const unixMatch = trimmed.match(/^(\d{9,13})\s*[-–]\s*(\d{9,13})$/);
  if (unixMatch) {
    let from = parseInt(unixMatch[1]);
    let to = parseInt(unixMatch[2]);
    if (from < 1e12) from *= 1000;
    if (to < 1e12) to *= 1000;
    return { from: new Date(from), to: new Date(to) };
  }

  // Simple relative: "45m", "12h", "2d", "1w", "1mo"
  const relMatch = trimmed.match(/^(\d+)\s*(m|min|h|hour|d|day|w|week|mo|month)s?$/i);
  if (relMatch) {
    const n = parseInt(relMatch[1]);
    const unit = relMatch[2].toLowerCase();
    const msMap: Record<string, number> = {
      m: 60000, min: 60000, h: 3600000, hour: 3600000,
      d: 86400000, day: 86400000, w: 604800000, week: 604800000,
      mo: 2592000000, month: 2592000000,
    };
    return { from: new Date(Date.now() - n * (msMap[unit] ?? 3600000)), to: new Date() };
  }

  // "since X" (growing range)
  const sinceMatch = trimmed.match(/^since\s+(.+)$/i);
  if (sinceMatch) {
    const parsed = chrono.parseDate(sinceMatch[1]);
    if (parsed) return { from: parsed, to: new Date() };
  }

  // "X to now"
  const toNowMatch = trimmed.match(/^(.+)\s+to\s+now$/i);
  if (toNowMatch) {
    const parsed = chrono.parseDate(toNowMatch[1]);
    if (parsed) return { from: parsed, to: new Date() };
  }

  // Range: "X - Y" or "X – Y"
  const rangeParts = trimmed.split(/\s*[-–]\s*/);
  if (rangeParts.length === 2 && rangeParts[0] && rangeParts[1]) {
    const from = chrono.parseDate(rangeParts[0]);
    const to = chrono.parseDate(rangeParts[1]);
    if (from && to) return { from, to };
  }

  // Single date/expression
  const parsed = chrono.parseDate(trimmed);
  if (parsed) return { from: parsed, to: new Date() };

  return null;
}

function formatDateRange(from: Date, to: Date): string {
  const opts: Intl.DateTimeFormatOptions = { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" };
  const sameDay = from.toDateString() === to.toDateString();
  if (sameDay) {
    const timeOpts: Intl.DateTimeFormatOptions = { hour: "numeric", minute: "2-digit" };
    return `${from.toLocaleDateString("en-US", { month: "short", day: "numeric" })}, ${from.toLocaleTimeString("en-US", timeOpts)} – ${to.toLocaleTimeString("en-US", timeOpts)}`;
  }
  return `${from.toLocaleDateString("en-US", opts)} – ${to.toLocaleDateString("en-US", opts)}`;
}

function getTimezone(): string {
  try {
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const offset = new Date().getTimezoneOffset();
    const sign = offset <= 0 ? "+" : "-";
    const h = String(Math.floor(Math.abs(offset) / 60)).padStart(2, "0");
    const m = String(Math.abs(offset) % 60).padStart(2, "0");
    return `${tz} (UTC${sign}${h}:${m})`;
  } catch {
    return "Local";
  }
}

function parseDurationMs(d: string): number {
  const match = d.match(/^(\d+)(m|h|d)$/);
  if (!match) return 3600000;
  const n = parseInt(match[1]);
  const unit = match[2];
  if (unit === "m") return n * 60000;
  if (unit === "h") return n * 3600000;
  return n * 86400000;
}

export function MetricsPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [selectedPreset, setSelectedPreset] = useState<string>("1h");
  const [customRange, setCustomRange] = useState<{ from: string; to: string } | null>(null);
  const [inputValue, setInputValue] = useState("");
  const [showMore, setShowMore] = useState(false);
  const [showCalendar, setShowCalendar] = useState(false);
  const [calFrom, setCalFrom] = useState("");
  const [calTo, setCalTo] = useState("");

  const isLive = selectedPreset !== null && !customRange;
  const activePreset = [...PRESETS, ...MORE_PRESETS].find((p) => p.key === selectedPreset);

  const apiRange = customRange ? undefined : (activePreset?.duration ?? "1h");
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["metrics", clusterName, customRange ? `custom:${customRange.from}:${customRange.to}` : apiRange],
    queryFn: () =>
      customRange
        ? api.metrics.get(clusterName!, undefined, customRange.from, customRange.to)
        : api.metrics.get(clusterName!, apiRange),
    refetchInterval: isLive ? 30000 : false,
  });

  function handlePresetSelect(preset: typeof PRESETS[number] | typeof MORE_PRESETS[number]) {
    setSelectedPreset(preset.key);
    setCustomRange(null);
    setInputValue("");
    setPickerOpen(false);
  }

  function handleInputSubmit() {
    const result = parseTimeInput(inputValue);
    if (result) {
      setCustomRange({ from: result.from.toISOString(), to: result.to.toISOString() });
      setSelectedPreset(null as any);
      setPickerOpen(false);
    }
  }

  function handleExampleClick(example: string) {
    setInputValue(example);
    const result = parseTimeInput(example);
    if (result) {
      setCustomRange({ from: result.from.toISOString(), to: result.to.toISOString() });
      setSelectedPreset(null as any);
      setPickerOpen(false);
    }
  }

  function handleCalendarApply() {
    if (!calFrom) return;
    const from = new Date(calFrom).toISOString();
    const to = calTo ? new Date(calTo).toISOString() : new Date().toISOString();
    setCustomRange({ from, to });
    setSelectedPreset(null as any);
    setPickerOpen(false);
  }

  const displayLabel = customRange
    ? formatDateRange(new Date(customRange.from), new Date(customRange.to))
    : activePreset?.label ?? "Past 1 Hour";

  if (isLoading) return <TableSkeleton rows={3} cols={4} />;

  if (error) {
    const msg = (error as Error).message;
    if (msg.includes("not configured")) {
      return (
        <div>
          <PageHeader
            title="Metrics"
            breadcrumbs={[
              { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
              { label: "Metrics" },
            ]}
          />
          <EmptyState
            icon={BarChart3}
            title="Metrics not configured"
            description="Add a metrics URL to this cluster's configuration to enable broker metrics. Requires Prometheus JMX Exporter on each broker."
          />
        </div>
      );
    }
    return <ErrorAlert message={msg} onRetry={() => refetch()} />;
  }

  const brokers = data?.brokers ?? [];

  return (
    <div>
      <PageHeader
        title="Metrics"
        breadcrumbs={[
          { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
          { label: "Metrics" },
        ]}
        description="Broker metrics from Prometheus JMX Exporter (auto-refreshes every 30s)"
      />

      <div className="mb-4">
        <Popover open={pickerOpen} onOpenChange={setPickerOpen}>
          <PopoverTrigger asChild>
            <Button variant="outline" className="h-9 gap-2 px-3 text-sm font-normal">
              {isLive && (
                <span className="flex items-center gap-1.5">
                  <span className="relative flex h-2 w-2">
                    <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75" />
                    <span className="relative inline-flex h-2 w-2 rounded-full bg-green-500" />
                  </span>
                  <span className="text-green-500 font-medium text-xs">LIVE</span>
                </span>
              )}
              <span>{displayLabel}</span>
              <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-[620px] p-0" align="start" sideOffset={8}>
            <div className="flex">
              {/* Left column: input + help */}
              <div className="flex-1 border-r p-4">
                <div className="mb-4">
                  <Input
                    placeholder="e.g. 45m, Mar 1 - Mar 7, since yesterday"
                    value={inputValue}
                    onChange={(e) => setInputValue(e.target.value)}
                    onKeyDown={(e) => { if (e.key === "Enter") handleInputSubmit(); }}
                    className="h-8 text-sm"
                  />
                </div>

                <p className="text-xs text-muted-foreground font-medium mb-3">Type custom times like:</p>

                <div className="space-y-3">
                  <div>
                    <p className="text-[10px] uppercase tracking-wider text-muted-foreground mb-1.5">Relative</p>
                    <div className="flex flex-wrap gap-1.5">
                      {["45m", "12 hours", "10d", "2 weeks", "last month", "yesterday", "today"].map((ex) => (
                        <button key={ex} onClick={() => handleExampleClick(ex)} className="text-xs text-primary hover:underline cursor-pointer px-1.5 py-0.5 rounded bg-muted/50 hover:bg-muted transition-colors">
                          {ex}
                        </button>
                      ))}
                    </div>
                  </div>

                  <div>
                    <p className="text-[10px] uppercase tracking-wider text-muted-foreground mb-1.5">Fixed</p>
                    <div className="flex flex-wrap gap-1.5">
                      {["Mar 1", "Mar 1 – Mar 2", "3/1", "3/1 – 3/2 12:00pm – 6:00pm"].map((ex) => (
                        <button key={ex} onClick={() => handleExampleClick(ex)} className="text-xs text-primary hover:underline cursor-pointer px-1.5 py-0.5 rounded bg-muted/50 hover:bg-muted transition-colors">
                          {ex}
                        </button>
                      ))}
                    </div>
                  </div>

                  <div>
                    <p className="text-[10px] uppercase tracking-wider text-muted-foreground mb-1.5">Growing</p>
                    <div className="flex flex-wrap gap-1.5">
                      {["since 3/1", "Mar 2 12pm to now"].map((ex) => (
                        <button key={ex} onClick={() => handleExampleClick(ex)} className="text-xs text-primary hover:underline cursor-pointer px-1.5 py-0.5 rounded bg-muted/50 hover:bg-muted transition-colors">
                          {ex}
                        </button>
                      ))}
                    </div>
                  </div>

                  <div>
                    <p className="text-[10px] uppercase tracking-wider text-muted-foreground mb-1.5">Unix timestamps</p>
                    <div className="flex flex-wrap gap-1.5">
                      {["1772299807 – 1772904607"].map((ex) => (
                        <button key={ex} onClick={() => handleExampleClick(ex)} className="text-xs text-primary hover:underline cursor-pointer px-1.5 py-0.5 rounded bg-muted/50 hover:bg-muted transition-colors">
                          {ex}
                        </button>
                      ))}
                    </div>
                  </div>
                </div>
              </div>

              {/* Right column: presets */}
              <div className="w-[240px] p-3">
                <p className="text-[10px] text-muted-foreground mb-1 text-right">{getTimezone()}</p>
                <p className="text-xs font-medium mb-2 text-right">
                  {customRange
                    ? formatDateRange(new Date(customRange.from), new Date(customRange.to))
                    : formatDateRange(new Date(Date.now() - (activePreset ? parseDurationMs(activePreset.duration) : 3600000)), new Date())}
                </p>

                <div className="border-t pt-2 space-y-0.5">
                  {PRESETS.map((p) => (
                    <button
                      key={p.key}
                      onClick={() => handlePresetSelect(p)}
                      className={`w-full flex items-center gap-2 px-2 py-1.5 rounded text-sm transition-colors ${
                        selectedPreset === p.key && !customRange ? "bg-accent" : "hover:bg-accent/50"
                      }`}
                    >
                      <span className={`inline-flex items-center justify-center min-w-[36px] px-1.5 py-0.5 rounded text-[10px] font-mono font-medium ${
                        "live" in p && p.live
                          ? "bg-green-500/20 text-green-500"
                          : "bg-muted text-muted-foreground"
                      }`}>
                        {"live" in p && p.live && (
                          <span className="inline-flex h-1.5 w-1.5 rounded-full bg-green-500 mr-1" />
                        )}
                        {p.shorthand}
                      </span>
                      <span className="text-xs">{p.label}</span>
                    </button>
                  ))}

                  {showMore && MORE_PRESETS.map((p) => (
                    <button
                      key={p.key}
                      onClick={() => handlePresetSelect(p)}
                      className={`w-full flex items-center gap-2 px-2 py-1.5 rounded text-sm transition-colors ${
                        selectedPreset === p.key && !customRange ? "bg-accent" : "hover:bg-accent/50"
                      }`}
                    >
                      <span className="inline-flex items-center justify-center min-w-[36px] px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-muted text-muted-foreground">
                        {p.shorthand}
                      </span>
                      <span className="text-xs">{p.label}</span>
                    </button>
                  ))}
                </div>

                <div className="border-t mt-2 pt-2 space-y-1">
                  <button
                    onClick={() => setShowCalendar(!showCalendar)}
                    className="w-full flex items-center gap-2 px-2 py-1.5 rounded text-sm hover:bg-accent/50 transition-colors"
                  >
                    <Calendar className="h-3.5 w-3.5 text-muted-foreground" />
                    <span className="text-xs">Select from calendar...</span>
                  </button>

                  {showCalendar && (
                    <div className="px-2 py-2 space-y-2">
                      <Input type="datetime-local" value={calFrom} onChange={(e) => setCalFrom(e.target.value)} className="h-7 text-xs" />
                      <Input type="datetime-local" value={calTo} onChange={(e) => setCalTo(e.target.value)} className="h-7 text-xs" />
                      <Button size="sm" className="w-full h-7 text-xs" disabled={!calFrom} onClick={handleCalendarApply}>
                        Apply
                      </Button>
                    </div>
                  )}

                  <button
                    onClick={() => setShowMore(!showMore)}
                    className="w-full flex items-center gap-2 px-2 py-1.5 rounded text-sm hover:bg-accent/50 transition-colors"
                  >
                    <span className="inline-flex items-center justify-center min-w-[36px] px-1.5 py-0.5 rounded text-[10px] font-mono bg-muted text-muted-foreground">•••</span>
                    <span className="text-xs">More</span>
                  </button>
                </div>
              </div>
            </div>
          </PopoverContent>
        </Popover>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {brokers.map((broker) => (
          <BrokerSection key={broker.id} broker={broker} range_={effectiveRange(customRange, apiRange ?? "1h")} />
        ))}
      </div>
    </div>
  );
}
