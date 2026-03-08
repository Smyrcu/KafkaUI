import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Link } from "react-router-dom";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Database } from "lucide-react";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";

export function ClustersPage() {
  const { data: clusters, isLoading, error, refetch } = useQuery({
    queryKey: ["clusters"],
    queryFn: api.clusters.list,
  });

  if (isLoading) return <><PageHeader title="Clusters" breadcrumbs={[{ label: "Dashboard", href: "/" }, { label: "Clusters" }]} /><TableSkeleton rows={3} cols={2} /></>;
  if (error) return <ErrorAlert error={error} onRetry={() => refetch()} />;

  return (
    <div>
      <PageHeader
        title="Clusters"
        description="All configured Kafka clusters"
        breadcrumbs={[
          { label: "Dashboard", href: "/" },
          { label: "Clusters" },
        ]}
      />
      {clusters?.length === 0 ? (
        <EmptyState
          icon={Database}
          title="No clusters found"
          description="Configure Kafka clusters in your application settings."
        />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {clusters?.map((cluster) => (
            <Link key={cluster.name} to={`/clusters/${cluster.name}/brokers`}>
              <Card className="hover:border-primary transition-colors cursor-pointer animate-scale-in">
                <CardHeader className="flex flex-row items-center gap-3">
                  <div className="rounded-md bg-primary/10 p-2 text-primary">
                    <Database className="h-5 w-5" />
                  </div>
                  <CardTitle className="text-lg">{cluster.name}</CardTitle>
                </CardHeader>
                <CardContent>
                  <Badge variant="secondary">{cluster.bootstrapServers}</Badge>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
