import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";
import { StatCard } from "@/components/StatCard";
import { EmptyState } from "@/components/EmptyState";
import { DashboardSkeleton } from "@/components/PageSkeleton";
import { Database, Server, FileText, Activity, Layers } from "lucide-react";

function statusBadge(status: string) {
  switch (status) {
    case "degraded":
      return <Badge variant="warning">{status}</Badge>;
    case "unreachable":
      return <Badge variant="destructive">{status}</Badge>;
    default:
      return <Badge variant="success">{status}</Badge>;
  }
}

export function DashboardPage() {
  const { data: overview, isLoading, error, refetch } = useQuery({
    queryKey: ["dashboard"],
    queryFn: () => api.dashboard.overview(),
    refetchInterval: 30000,
  });

  if (isLoading) return <DashboardSkeleton />;
  if (error) return <><PageHeader title="Dashboard" description="Overview of your Kafka clusters" /><ErrorAlert error={error} onRetry={() => refetch()} /></>;

  const clusters = overview ?? [];

  const totalBrokers = clusters.reduce((sum, c) => sum + c.brokerCount, 0);
  const totalTopics = clusters.reduce((sum, c) => sum + c.topicCount, 0);
  const totalGroups = clusters.reduce((sum, c) => sum + c.consumerGroupCount, 0);

  return (
    <div>
      <PageHeader
        title="Dashboard"
        description="Overview of your Kafka clusters"
      />

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <StatCard label="Clusters" value={clusters.length} icon={Layers} accent="primary" />
        <StatCard label="Brokers" value={totalBrokers} icon={Server} accent="success" />
        <StatCard label="Topics" value={totalTopics} icon={FileText} accent="warning" />
        <StatCard label="Consumer Groups" value={totalGroups} icon={Activity} accent="primary" />
      </div>

      {clusters.length === 0 ? (
        <EmptyState
          icon={Database}
          title="No clusters configured"
          description="Add a Kafka cluster to your configuration to get started."
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {clusters.map((cluster) => (
            <Card key={cluster.name} className="animate-scale-in">
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-lg">
                    <Link
                      to={`/clusters/${encodeURIComponent(cluster.name)}/topics`}
                      className="text-primary hover:underline"
                    >
                      {cluster.name}
                    </Link>
                  </CardTitle>
                  {statusBadge(cluster.status)}
                </div>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-3 gap-4 mb-3">
                  <div className="text-center">
                    <Link
                      to={`/clusters/${encodeURIComponent(cluster.name)}/brokers`}
                      className="text-primary hover:underline"
                    >
                      <p className="text-xl font-semibold">{cluster.brokerCount}</p>
                      <p className="text-xs text-muted-foreground">Brokers</p>
                    </Link>
                  </div>
                  <div className="text-center">
                    <Link
                      to={`/clusters/${encodeURIComponent(cluster.name)}/topics`}
                      className="text-primary hover:underline"
                    >
                      <p className="text-xl font-semibold">{cluster.topicCount}</p>
                      <p className="text-xs text-muted-foreground">Topics</p>
                    </Link>
                  </div>
                  <div className="text-center">
                    <Link
                      to={`/clusters/${encodeURIComponent(cluster.name)}/consumer-groups`}
                      className="text-primary hover:underline"
                    >
                      <p className="text-xl font-semibold">{cluster.consumerGroupCount}</p>
                      <p className="text-xs text-muted-foreground">Groups</p>
                    </Link>
                  </div>
                </div>
                <p className="text-xs text-muted-foreground truncate" title={cluster.bootstrapServers}>
                  {cluster.bootstrapServers}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
