import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useState, useRef } from "react";
import * as chrono from "chrono-node";
import { api } from "@/lib/api";
import type { MetricGroup, MetricDetail, MetricHistoryPoint } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorAlert } from "@/components/ErrorAlert";
import { EmptyState } from "@/components/EmptyState";
import { PageHeader } from "@/components/PageHeader";
import { TableSkeleton } from "@/components/PageSkeleton";
import { Input } from "@/components/ui/input";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { BarChart3, Calendar, ChevronDown, ChevronRight } from "lucide-react";
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from "recharts";
import { getErrorMessage } from "@/lib/error-utils";

const PRESETS = [
  { key: "live", shorthand: "LIVE", label: "Live", duration: "live", live: true },
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

function formatCompactNumber(value: number): string {
  if (Math.abs(value) >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(1)}G`;
  if (Math.abs(value) >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (Math.abs(value) >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
  if (Number.isInteger(value)) return value.toString();
  return value.toFixed(2);
}

function formatMetricValue(value: number, metricName: string): string {
  if (metricName.includes("bytes") || metricName.includes("memory")) {
    if (Math.abs(value) >= 1_073_741_824) return `${(value / 1_073_741_824).toFixed(1)} GB`;
    if (Math.abs(value) >= 1_048_576) return `${(value / 1_048_576).toFixed(1)} MB`;
    if (Math.abs(value) >= 1024) return `${(value / 1024).toFixed(1)} KB`;
    return `${value.toFixed(0)} B`;
  }
  if (metricName.includes("seconds") || metricName.includes("duration")) {
    if (value >= 3600) return `${(value / 3600).toFixed(1)}h`;
    if (value >= 60) return `${(value / 60).toFixed(1)}m`;
    return `${value.toFixed(1)}s`;
  }
  if (metricName.includes("milliseconds") || metricName.includes("_ms")) {
    if (value >= 60000) return `${(value / 60000).toFixed(1)}m`;
    if (value >= 1000) return `${(value / 1000).toFixed(1)}s`;
    return `${value.toFixed(0)}ms`;
  }
  if (metricName.includes("rate") || metricName.includes("persec")) {
    return formatCompactNumber(value) + "/s";
  }
  return formatCompactNumber(value);
}

function formatTime(iso: string, range_: string): string {
  const d = new Date(iso);
  if (["2d", "3d", "7d", "14d", "15d", "30d"].includes(range_)) {
    return d.toLocaleDateString("en-GB", { day: "2-digit", month: "short" });
  }
  if (["4h", "6h", "12h", "24h", "1d"].includes(range_)) {
    return d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" });
  }
  return d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function effectiveRange(custom: { from: string; to: string } | null, preset: string): string {
  if (!custom) return preset;
  const ms = new Date(custom.to).getTime() - new Date(custom.from).getTime();
  const hours = ms / 3600000;
  if (hours >= 168) return "30d";
  if (hours >= 72) return "7d";
  if (hours >= 24) return "1d";
  if (hours >= 4) return "4h";
  return "1h";
}

function parseTimeInput(input: string): { from: Date; to: Date } | null {
  const trimmed = input.trim();
  if (!trimmed) return null;

  const unixMatch = trimmed.match(/^(\d{9,13})\s*[-–]\s*(\d{9,13})$/);
  if (unixMatch) {
    let from = parseInt(unixMatch[1]);
    let to = parseInt(unixMatch[2]);
    if (from < 1e12) from *= 1000;
    if (to < 1e12) to *= 1000;
    return { from: new Date(from), to: new Date(to) };
  }

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

  const sinceMatch = trimmed.match(/^since\s+(.+)$/i);
  if (sinceMatch) {
    const parsed = chrono.parseDate(sinceMatch[1]);
    if (parsed) return { from: parsed, to: new Date() };
  }

  const toNowMatch = trimmed.match(/^(.+)\s+to\s+now$/i);
  if (toNowMatch) {
    const parsed = chrono.parseDate(toNowMatch[1]);
    if (parsed) return { from: parsed, to: new Date() };
  }

  const dashIdx = trimmed.search(/\s+[-–]\s+/);
  if (dashIdx > 0) {
    const sep = trimmed.slice(dashIdx).match(/^\s+[-–]\s+/);
    if (sep) {
      const fromStr = trimmed.slice(0, dashIdx);
      const toStr = trimmed.slice(dashIdx + sep[0].length);
      const from = chrono.parseDate(fromStr);
      const to = chrono.parseDate(toStr);
      if (from && to) return { from, to };
    }
  }

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

// ── Generic Chart ────────────────────────────────────────

function GenericChart({ history, range_, name }: { history: MetricHistoryPoint[]; range_: string; name: string }) {
  const data = history.map((p) => ({
    time: formatTime(p.time, range_),
    value: p.value,
  }));

  return (
    <ResponsiveContainer width="100%" height={180}>
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" className="opacity-30" />
        <XAxis dataKey="time" tick={{ fontSize: 11 }} interval="preserveStartEnd" />
        <YAxis tick={{ fontSize: 11 }} width={65} tickFormatter={(v) => formatCompactNumber(v)} />
        <Tooltip
          formatter={(value: number | undefined) => [formatMetricValue(value ?? 0, name), name]}
          contentStyle={{
            fontSize: 12,
            backgroundColor: "hsl(var(--card))",
            border: "1px solid hsl(var(--border))",
            borderRadius: 6,
          }}
        />
        <Line type="monotone" dataKey="value" stroke="#60a5fa" strokeWidth={2} dot={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}

// ── Metric Row ───────────────────────────────────────────

function MetricRow({ metric, prefix, selected, onSelect, range_ }: {
  metric: MetricDetail;
  prefix: string;
  selected: boolean;
  onSelect: () => void;
  range_: string;
}) {
  const shortName = metric.name.replace(prefix, "");
  const primarySample = metric.current[0];

  return (
    <>
      <tr onClick={onSelect} className="cursor-pointer hover:bg-muted/50 border-t">
        <td className="py-2 font-mono text-xs" title={metric.help}>
          {shortName}
        </td>
        <td className="py-2">
          {primarySample?.labels && Object.keys(primarySample.labels).length > 0 && (
            <div className="flex flex-wrap gap-1">
              {Object.entries(primarySample.labels).map(([k, v]) => (
                <Badge key={k} variant="outline" className="text-[10px] font-mono">
                  {k}={v}
                </Badge>
              ))}
            </div>
          )}
        </td>
        <td className="py-2 text-right font-mono text-xs">
          <Badge variant="secondary">{formatMetricValue(primarySample?.value ?? 0, metric.name)}</Badge>
        </td>
        <td className="py-2 pl-2 text-right">
          <Badge variant="outline" className="text-[10px]">{metric.type}</Badge>
        </td>
      </tr>
      {selected && metric.history.length >= 2 && (
        <tr>
          <td colSpan={4} className="pb-4 pt-2">
            <GenericChart history={metric.history} range_={range_} name={shortName} />
          </td>
        </tr>
      )}
    </>
  );
}

// ── Metric Group Section ─────────────────────────────────

function MetricGroupSection({ group, range_ }: { group: MetricGroup; range_: string }) {
  const [expanded, setExpanded] = useState(true);
  const [selectedMetric, setSelectedMetric] = useState<string | null>(null);

  return (
    <Card className="animate-scale-in">
      <CardHeader onClick={() => setExpanded(!expanded)} className="cursor-pointer py-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base flex items-center gap-2">
            {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
            {group.name}
          </CardTitle>
          <Badge variant="secondary">{group.metrics.length} metrics</Badge>
        </div>
      </CardHeader>
      {expanded && (
        <CardContent className="pt-0">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs text-muted-foreground">
                <th className="pb-2">Metric</th>
                <th className="pb-2">Labels</th>
                <th className="pb-2 text-right">Value</th>
                <th className="pb-2 text-right">Type</th>
              </tr>
            </thead>
            <tbody>
              {group.metrics.map((m) => (
                <MetricRow
                  key={m.name}
                  metric={m}
                  prefix={group.prefix}
                  selected={selectedMetric === m.name}
                  onSelect={() => setSelectedMetric(selectedMetric === m.name ? null : m.name)}
                  range_={range_}
                />
              ))}
            </tbody>
          </table>
        </CardContent>
      )}
    </Card>
  );
}

// ── Main Page ────────────────────────────────────────────

export function MetricsPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [selectedPreset, setSelectedPreset] = useState<string | null>("1h");
  const [customRange, setCustomRange] = useState<{ from: string; to: string } | null>(null);
  const [inputValue, setInputValue] = useState("");
  const [showMore, setShowMore] = useState(false);
  const [showCalendar, setShowCalendar] = useState(false);
  const [calFrom, setCalFrom] = useState("");
  const [calTo, setCalTo] = useState("");
  const [search, setSearch] = useState("");
  const liveStartRef = useRef<string | null>(null);

  const isLive = selectedPreset === "live";
  const activePreset = [...PRESETS, ...MORE_PRESETS].find((p) => p.key === selectedPreset);
  const isAutoRefresh = selectedPreset !== null && !customRange;

  const apiRange = customRange ? undefined : (isLive ? undefined : (activePreset?.duration ?? "1h"));
  const liveFrom = liveStartRef.current;
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["metrics", clusterName, isLive ? `live:${liveFrom}` : customRange ? `custom:${customRange.from}:${customRange.to}` : apiRange],
    queryFn: () => {
      if (isLive && liveFrom) {
        return api.metrics.get(clusterName!, undefined, liveFrom, new Date().toISOString());
      }
      if (customRange) {
        return api.metrics.get(clusterName!, undefined, customRange.from, customRange.to);
      }
      return api.metrics.get(clusterName!, apiRange);
    },
    enabled: !!clusterName,
    refetchInterval: isAutoRefresh ? (isLive ? 5000 : 30000) : false,
  });

  function handlePresetSelect(preset: typeof PRESETS[number] | typeof MORE_PRESETS[number]) {
    setSelectedPreset(preset.key);
    setCustomRange(null);
    setInputValue("");
    if ("live" in preset && preset.live) {
      liveStartRef.current = new Date().toISOString();
    } else {
      liveStartRef.current = null;
    }
    setPickerOpen(false);
  }

  function handleInputSubmit() {
    const result = parseTimeInput(inputValue);
    if (result) {
      setCustomRange({ from: result.from.toISOString(), to: result.to.toISOString() });
      setSelectedPreset(null);
      setPickerOpen(false);
    }
  }

  function handleExampleClick(example: string) {
    setInputValue(example);
    const result = parseTimeInput(example);
    if (result) {
      setCustomRange({ from: result.from.toISOString(), to: result.to.toISOString() });
      setSelectedPreset(null);
      setPickerOpen(false);
    }
  }

  function handleCalendarApply() {
    if (!calFrom) return;
    const from = new Date(calFrom).toISOString();
    const to = calTo ? new Date(calTo).toISOString() : new Date().toISOString();
    setCustomRange({ from, to });
    setSelectedPreset(null);
    setPickerOpen(false);
  }

  const displayLabel = isLive
    ? (liveFrom ? `Since ${new Date(liveFrom).toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit" })}` : "Live")
    : customRange
      ? formatDateRange(new Date(customRange.from), new Date(customRange.to))
      : activePreset?.label ?? "Past 1 Hour";

  const breadcrumbs = [
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Metrics" },
  ];

  if (isLoading) return <><PageHeader title="Metrics" breadcrumbs={breadcrumbs} /><TableSkeleton rows={3} cols={4} /></>;

  if (error) {
    const msg = getErrorMessage(error);
    if (msg.includes("not configured")) {
      return (
        <div>
          <PageHeader title="Metrics" breadcrumbs={breadcrumbs} />
          <EmptyState
            icon={BarChart3}
            title="Metrics not configured"
            description="Add a metrics URL to this cluster's configuration to enable Prometheus metrics collection."
          />
        </div>
      );
    }
    return <><PageHeader title="Metrics" breadcrumbs={breadcrumbs} /><ErrorAlert error={error} onRetry={() => refetch()} /></>;
  }

  const groups = data?.groups ?? [];

  const filteredGroups = groups
    .map((g) => ({
      ...g,
      metrics: g.metrics.filter((m) =>
        m.name.toLowerCase().includes(search.toLowerCase()) ||
        m.help.toLowerCase().includes(search.toLowerCase()) ||
        (m.current[0]?.labels && JSON.stringify(m.current[0].labels).toLowerCase().includes(search.toLowerCase()))
      ),
    }))
    .filter((g) => g.metrics.length > 0);

  const currentRange = isLive
    ? effectiveRange(liveFrom ? { from: liveFrom, to: new Date().toISOString() } : null, "1h")
    : effectiveRange(customRange, apiRange ?? "1h");

  return (
    <div>
      <PageHeader
        title="Metrics"
        breadcrumbs={breadcrumbs}
        description="Prometheus metrics from configured endpoint"
      />

      <div className="flex gap-3 mb-4">
        <Input
          placeholder="Search metrics..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-md h-9"
        />

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
                      {["Mar 1", "Mar 1 – Mar 2", "3/1", "3/1 12pm – 3/2 6pm"].map((ex) => (
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
                  {isLive && liveFrom
                    ? `Since ${new Date(liveFrom).toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit", second: "2-digit" })}`
                    : customRange
                      ? formatDateRange(new Date(customRange.from), new Date(customRange.to))
                      : formatDateRange(new Date(Date.now() - (activePreset ? parseDurationMs(activePreset.duration) : 3600000)), new Date())}
                </p>

                <div className="border-t pt-2 space-y-0.5">
                  {PRESETS.map((p) => (
                    <button
                      key={p.key}
                      onClick={() => handlePresetSelect(p)}
                      aria-current={selectedPreset === p.key && !customRange ? "true" : undefined}
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
                      aria-current={selectedPreset === p.key && !customRange ? "true" : undefined}
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
                    aria-expanded={showCalendar}
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
                    aria-expanded={showMore}
                    className="w-full flex items-center gap-2 px-2 py-1.5 rounded text-sm hover:bg-accent/50 transition-colors"
                  >
                    <span className="inline-flex items-center justify-center min-w-[36px] px-1.5 py-0.5 rounded text-[10px] font-mono bg-muted text-muted-foreground">...</span>
                    <span className="text-xs">More</span>
                  </button>
                </div>
              </div>
            </div>
          </PopoverContent>
        </Popover>
      </div>

      {filteredGroups.length === 0 ? (
        <EmptyState
          icon={BarChart3}
          title={search ? "No metrics match" : "No metrics data yet"}
          description={search ? "Try a different search term." : "Collecting data... metrics will appear shortly."}
        />
      ) : (
        <div className="space-y-4">
          {filteredGroups.map((group) => (
            <MetricGroupSection key={group.prefix} group={group} range_={currentRange} />
          ))}
        </div>
      )}
    </div>
  );
}
