import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
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
import { BarChart3, ArrowDownToLine, ArrowUpFromLine, Mail, AlertTriangle, Crown, WifiOff, Calendar } from "lucide-react";
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from "recharts";

const TIME_RANGES = [
  { label: "1m", value: "1m" },
  { label: "5m", value: "5m" },
  { label: "15m", value: "15m" },
  { label: "30m", value: "30m" },
  { label: "1h", value: "1h" },
  { label: "3h", value: "3h" },
  { label: "6h", value: "6h" },
  { label: "12h", value: "12h" },
  { label: "1d", value: "1d" },
  { label: "3d", value: "3d" },
  { label: "7d", value: "7d" },
  { label: "14d", value: "14d" },
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

export function MetricsPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const [range_, setRange] = useState("1h");
  const [customMode, setCustomMode] = useState(false);
  const [customFrom, setCustomFrom] = useState("");
  const [customTo, setCustomTo] = useState("");
  const [appliedCustom, setAppliedCustom] = useState<{ from: string; to: string } | null>(null);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["metrics", clusterName, appliedCustom ? `custom:${appliedCustom.from}:${appliedCustom.to}` : range_],
    queryFn: () =>
      appliedCustom
        ? api.metrics.get(clusterName!, undefined, appliedCustom.from, appliedCustom.to)
        : api.metrics.get(clusterName!, range_),
    refetchInterval: appliedCustom ? false : 30000,
  });

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

      <div className="flex flex-wrap items-center gap-1 mb-4">
        {TIME_RANGES.map((r) => (
          <Button
            key={r.value}
            variant={!appliedCustom && range_ === r.value ? "default" : "outline"}
            size="sm"
            onClick={() => {
              setRange(r.value);
              setAppliedCustom(null);
              setCustomMode(false);
            }}
          >
            {r.label}
          </Button>
        ))}
        <Button
          variant={customMode || appliedCustom ? "default" : "outline"}
          size="sm"
          onClick={() => setCustomMode(!customMode)}
        >
          <Calendar className="h-3.5 w-3.5 mr-1" />
          Custom
        </Button>
      </div>

      {customMode && (
        <div className="flex items-end gap-2 mb-4">
          <div>
            <label className="text-xs text-muted-foreground mb-1 block">From</label>
            <Input
              type="datetime-local"
              value={customFrom}
              onChange={(e) => setCustomFrom(e.target.value)}
              className="w-48 h-8 text-sm"
            />
          </div>
          <div>
            <label className="text-xs text-muted-foreground mb-1 block">To</label>
            <Input
              type="datetime-local"
              value={customTo}
              onChange={(e) => setCustomTo(e.target.value)}
              className="w-48 h-8 text-sm"
            />
          </div>
          <Button
            size="sm"
            disabled={!customFrom}
            onClick={() => {
              const from = new Date(customFrom).toISOString();
              const to = customTo ? new Date(customTo).toISOString() : new Date().toISOString();
              setAppliedCustom({ from, to });
            }}
          >
            Apply
          </Button>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {brokers.map((broker) => (
          <BrokerSection key={broker.id} broker={broker} range_={effectiveRange(appliedCustom, range_)} />
        ))}
      </div>
    </div>
  );
}
