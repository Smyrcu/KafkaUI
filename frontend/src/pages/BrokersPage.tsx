import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";
import { DataTable } from "@/components/DataTable";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";
import { Server } from "lucide-react";
import type { BrokerInfo } from "@/lib/api";

export function BrokersPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const { data: brokers, isLoading, error, refetch } = useQuery({
    queryKey: ["brokers", clusterName],
    queryFn: () => api.brokers.list(clusterName!),
    enabled: !!clusterName,
  });

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Brokers" },
  ];

  if (isLoading) return <><PageHeader title="Brokers" breadcrumbs={breadcrumbs} /><TableSkeleton /></>;
  if (error) return <><PageHeader title="Brokers" breadcrumbs={breadcrumbs} /><ErrorAlert error={error} onRetry={() => refetch()} /></>;

  return (
    <div>
      <PageHeader title="Brokers" description={`Broker nodes in ${clusterName}`} breadcrumbs={breadcrumbs} />
      {brokers?.length === 0 ? (
        <EmptyState icon={Server} title="No brokers found" description="Unable to retrieve broker information from this cluster." />
      ) : (
        <DataTable<BrokerInfo>
          itemName="brokers"
          data={brokers ?? []}
          columns={[
            { header: "ID", cell: (b) => <Badge variant="outline">{b.id}</Badge> },
            { header: "Host", accessorKey: "host" },
            { header: "Port", accessorKey: "port" },
            { header: "Rack", cell: (b) => b.rack || "\u2014" },
          ]}
        />
      )}
    </div>
  );
}
