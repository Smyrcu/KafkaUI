import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useParams, Link } from "react-router-dom";
import { api } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";
import { DataTable } from "@/components/DataTable";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";
import { Users } from "lucide-react";
import type { ConsumerGroupInfo } from "@/lib/api";

export function ConsumerGroupsPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const [search, setSearch] = useState("");

  const { data: groups, isLoading, error, refetch } = useQuery({
    queryKey: ["consumer-groups", clusterName],
    queryFn: () => api.consumerGroups.list(clusterName!),
    enabled: !!clusterName,
  });

  const filteredGroups = groups?.filter((g) =>
    g.name.toLowerCase().includes(search.toLowerCase())
  ) ?? [];

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Consumer Groups" },
  ];

  if (isLoading) return <><PageHeader title="Consumer Groups" breadcrumbs={breadcrumbs} /><TableSkeleton cols={5} /></>;
  if (error) return <ErrorAlert message={(error as Error).message} onRetry={() => refetch()} />;

  return (
    <div>
      <PageHeader title="Consumer Groups" description={`Consumer groups in ${clusterName}`} breadcrumbs={breadcrumbs} />
      <div className="mb-4">
        <Input
          placeholder="Search consumer groups..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>
      {filteredGroups.length === 0 ? (
        <EmptyState icon={Users} title="No consumer groups found" description={search ? "No groups match your search." : "This cluster has no consumer groups."} />
      ) : (
        <DataTable<ConsumerGroupInfo>
          itemName="consumer groups"
          data={filteredGroups}
          columns={[
            {
              header: "Name",
              cell: (g) => (
                <Link to={`/clusters/${clusterName}/consumer-groups/${encodeURIComponent(g.name)}`} className="text-primary hover:underline font-medium">
                  {g.name}
                </Link>
              ),
            },
            {
              header: "State",
              cell: (g) => (
                <Badge variant={g.state === "Stable" ? "success" : g.state === "Empty" ? "secondary" : "destructive"}>
                  {g.state}
                </Badge>
              ),
            },
            { header: "Members", accessorKey: "members" },
            { header: "Topics", accessorKey: "topics" },
            { header: "Coordinator", accessorKey: "coordinatorId" },
          ]}
        />
      )}
    </div>
  );
}
