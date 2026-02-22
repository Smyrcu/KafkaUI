import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link } from "react-router-dom";
import { api } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { ErrorAlert } from "@/components/ErrorAlert";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { PageHeader } from "@/components/PageHeader";
import { DataTable } from "@/components/DataTable";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";
import { PlugZap, Plus } from "lucide-react";
import type { ConnectorInfo } from "@/lib/api";

export function KafkaConnectPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [newConnectorName, setNewConnectorName] = useState("");
  const [newConnectorConfig, setNewConnectorConfig] = useState("");

  const { data: connectors, isLoading, error, refetch } = useQuery({
    queryKey: ["connectors", clusterName],
    queryFn: () => api.connect.list(clusterName!),
    enabled: !!clusterName,
  });

  const createMutation = useMutation({
    mutationFn: () => {
      const parsedConfig = JSON.parse(newConnectorConfig);
      if (typeof parsedConfig !== "object" || parsedConfig === null || Array.isArray(parsedConfig)) {
        throw new Error("Config must be a JSON object");
      }
      return api.connect.create(clusterName!, { name: newConnectorName, config: parsedConfig });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["connectors", clusterName] });
      setCreateOpen(false);
      setNewConnectorName("");
      setNewConnectorConfig("");
    },
  });

  const restartMutation = useMutation({
    mutationFn: (name: string) => api.connect.restart(clusterName!, name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["connectors", clusterName] }),
  });

  const pauseMutation = useMutation({
    mutationFn: (name: string) => api.connect.pause(clusterName!, name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["connectors", clusterName] }),
  });

  const resumeMutation = useMutation({
    mutationFn: (name: string) => api.connect.resume(clusterName!, name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["connectors", clusterName] }),
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) => api.connect.delete(clusterName!, name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["connectors", clusterName] }),
  });

  const filteredConnectors = connectors?.filter((c) =>
    c.name.toLowerCase().includes(search.toLowerCase())
  ) ?? [];

  const getStateBadgeVariant = (state: string) => {
    switch (state) {
      case "RUNNING": return "success" as const;
      case "PAUSED": return "secondary" as const;
      case "FAILED": return "destructive" as const;
      default: return "outline" as const;
    }
  };

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Kafka Connect" },
  ];

  if (isLoading) return <><PageHeader title="Kafka Connect" breadcrumbs={breadcrumbs} /><TableSkeleton cols={6} /></>;
  if (error) return <ErrorAlert message={(error as Error).message} onRetry={() => refetch()} />;

  return (
    <div>
      <PageHeader
        title="Kafka Connect"
        description={`Manage connectors in ${clusterName}`}
        breadcrumbs={breadcrumbs}
        actions={
          <Dialog open={createOpen} onOpenChange={setCreateOpen}>
            <DialogTrigger asChild>
              <Button><Plus className="h-4 w-4 mr-2" />Create Connector</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create Connector</DialogTitle>
              </DialogHeader>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  createMutation.mutate();
                }}
                className="space-y-4"
              >
                <div className="space-y-2">
                  <Label htmlFor="connectorName">Connector Name</Label>
                  <Input
                    id="connectorName"
                    placeholder="e.g. my-jdbc-source"
                    value={newConnectorName}
                    onChange={(e) => setNewConnectorName(e.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="connectorConfig">Connector Config (JSON)</Label>
                  <Textarea
                    id="connectorConfig"
                    placeholder={'{"connector.class": "...", "tasks.max": "1", ...}'}
                    value={newConnectorConfig}
                    onChange={(e) => setNewConnectorConfig(e.target.value)}
                    rows={10}
                    required
                  />
                </div>
                <Button type="submit" disabled={createMutation.isPending}>
                  {createMutation.isPending ? "Creating..." : "Create"}
                </Button>
                {createMutation.isError && (
                  <ErrorAlert message={(createMutation.error as Error).message} />
                )}
              </form>
            </DialogContent>
          </Dialog>
        }
      />
      <div className="mb-4">
        <Input
          placeholder="Search connectors..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>
      {filteredConnectors.length === 0 ? (
        <EmptyState icon={PlugZap} title="No connectors found" description={search ? "No connectors match your search." : "No connectors deployed yet."} actionLabel={!search ? "Create Connector" : undefined} onAction={!search ? () => setCreateOpen(true) : undefined} />
      ) : (
        <DataTable<ConnectorInfo>
          itemName="connectors"
          data={filteredConnectors}
          columns={[
            {
              header: "Name",
              cell: (c) => (
                <Link to={`/clusters/${clusterName}/connect/${encodeURIComponent(c.name)}`} className="text-primary hover:underline font-medium">
                  {c.name}
                </Link>
              ),
            },
            {
              header: "Type",
              cell: (c) => <Badge variant={c.type === "source" ? "default" : "secondary"}>{c.type}</Badge>,
            },
            {
              header: "State",
              cell: (c) => <Badge variant={getStateBadgeVariant(c.state)}>{c.state}</Badge>,
            },
            { header: "Worker ID", accessorKey: "workerId" },
            { header: "Connect Cluster", accessorKey: "connectCluster" },
            {
              header: "Actions",
              cell: (c) => (
                <div className="flex items-center gap-1">
                  <Button variant="outline" size="sm" disabled={restartMutation.isPending} onClick={(e) => { e.stopPropagation(); restartMutation.mutate(c.name); }}>
                    Restart
                  </Button>
                  {c.state === "RUNNING" ? (
                    <Button variant="outline" size="sm" disabled={pauseMutation.isPending} onClick={(e) => { e.stopPropagation(); pauseMutation.mutate(c.name); }}>
                      Pause
                    </Button>
                  ) : (
                    <Button variant="outline" size="sm" disabled={resumeMutation.isPending} onClick={(e) => { e.stopPropagation(); resumeMutation.mutate(c.name); }}>
                      Resume
                    </Button>
                  )}
                  <Button variant="destructive" size="sm" disabled={deleteMutation.isPending} onClick={(e) => { e.stopPropagation(); if (confirm(`Delete connector "${c.name}"?`)) deleteMutation.mutate(c.name); }}>
                    Delete
                  </Button>
                </div>
              ),
            },
          ]}
        />
      )}
    </div>
  );
}
