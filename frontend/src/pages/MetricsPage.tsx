import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { BrokerMetricsInfo } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ErrorAlert } from "@/components/ErrorAlert";
import { EmptyState } from "@/components/EmptyState";
import { PageHeader } from "@/components/PageHeader";
import { TableSkeleton } from "@/components/PageSkeleton";
import { BarChart3, ArrowDownToLine, ArrowUpFromLine, Mail, AlertTriangle, Crown, WifiOff } from "lucide-react";

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

function BrokerCard({ broker }: { broker: BrokerMetricsInfo }) {
  if (broker.error) {
    return (
      <Card className="animate-scale-in">
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
  const stats = [
    { label: "Bytes In", value: formatBytes(m.bytesInPerSec), icon: ArrowDownToLine, warn: false },
    { label: "Bytes Out", value: formatBytes(m.bytesOutPerSec), icon: ArrowUpFromLine, warn: false },
    { label: "Messages In", value: formatRate(m.messagesInPerSec), icon: Mail, warn: false },
    { label: "Under-replicated", value: String(m.underReplicatedPartitions), icon: AlertTriangle, warn: m.underReplicatedPartitions > 0 },
    { label: "Active Controller", value: String(m.activeControllerCount), icon: Crown, warn: false },
    { label: "Offline Partitions", value: String(m.offlinePartitionsCount), icon: WifiOff, warn: m.offlinePartitionsCount > 0 },
  ];

  return (
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
  );
}

export function MetricsPage() {
  const { clusterName } = useParams<{ clusterName: string }>();

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["metrics", clusterName],
    queryFn: () => api.metrics.get(clusterName!),
    refetchInterval: 30000,
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

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {brokers.map((broker) => (
          <BrokerCard key={broker.id} broker={broker} />
        ))}
      </div>
    </div>
  );
}
