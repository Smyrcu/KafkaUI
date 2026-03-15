import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, useNavigate } from "react-router-dom";
import { useHasAction } from "@/hooks/usePermissions";
import { api } from "@/lib/api";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";
import { StatCard } from "@/components/StatCard";
import { DetailSkeleton } from "@/components/PageSkeleton";
import { EmptyState } from "@/components/EmptyState";
import { Activity, PlugZap, Server, ListTodo } from "lucide-react";
import { getConnectorStateBadgeVariant } from "@/lib/helpers";
import { rowClassName } from "@/lib/utils";
import { getErrorMessage } from "@/lib/error-utils";
import { ConfirmDialog } from "@/components/ConfirmDialog";

export function ConnectorDetailPage() {
  const { clusterName, connectorName } = useParams<{ clusterName: string; connectorName: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const canManageConnectors = useHasAction("manage_connectors");
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [configJson, setConfigJson] = useState("");
  const [configError, setConfigError] = useState<string | null>(null);

  const { data: connector, isLoading, error, refetch } = useQuery({
    queryKey: ["connector-detail", clusterName, connectorName],
    queryFn: () => api.connect.details(clusterName!, connectorName!),
    enabled: !!clusterName && !!connectorName,
  });

  const restartMutation = useMutation({
    mutationFn: () => api.connect.restart(clusterName!, connectorName!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["connector-detail", clusterName, connectorName] }),
  });

  const pauseMutation = useMutation({
    mutationFn: () => api.connect.pause(clusterName!, connectorName!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["connector-detail", clusterName, connectorName] }),
  });

  const resumeMutation = useMutation({
    mutationFn: () => api.connect.resume(clusterName!, connectorName!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["connector-detail", clusterName, connectorName] }),
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.connect.delete(clusterName!, connectorName!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["connectors", clusterName] });
      navigate(`/clusters/${clusterName}/connect`);
    },
  });

  const updateConfigMutation = useMutation({
    mutationFn: (config: Record<string, string>) => api.connect.update(clusterName!, connectorName!, config),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["connector-detail", clusterName, connectorName] });
      setEditDialogOpen(false);
    },
  });

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Kafka Connect", href: `/clusters/${clusterName}/connect` },
    { label: connectorName! },
  ];

  if (isLoading) return <DetailSkeleton />;
  if (error) return <><PageHeader title={connectorName!} breadcrumbs={breadcrumbs} /><ErrorAlert error={error} onRetry={() => refetch()} /></>;
  if (!connector) return null;

  const isPaused = connector.state.toUpperCase() === "PAUSED";
  const sortedConfigKeys = Object.keys(connector.config).sort();

  function openEditDialog() {
    setConfigJson(JSON.stringify(connector!.config, null, 2));
    setEditDialogOpen(true);
  }

  function handleSaveConfig() {
    try {
      const parsedConfig = JSON.parse(configJson);
      setConfigError(null);
      updateConfigMutation.mutate(parsedConfig);
    } catch {
      setConfigError("Invalid JSON configuration");
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={connector.name}
        breadcrumbs={breadcrumbs}
        actions={
          <div className="flex items-center gap-2">
            <Badge variant={getConnectorStateBadgeVariant(connector.state)}>{connector.state.toUpperCase()}</Badge>
            <Badge variant="secondary">{connector.type}</Badge>
            {canManageConnectors && (
              <>
                <Button variant="outline" size="sm" onClick={() => restartMutation.mutate()} disabled={restartMutation.isPending}>
                  {restartMutation.isPending ? "Restarting..." : "Restart"}
                </Button>
                {isPaused ? (
                  <Button variant="outline" size="sm" onClick={() => resumeMutation.mutate()} disabled={resumeMutation.isPending}>
                    {resumeMutation.isPending ? "Resuming..." : "Resume"}
                  </Button>
                ) : (
                  <Button variant="outline" size="sm" onClick={() => pauseMutation.mutate()} disabled={pauseMutation.isPending}>
                    {pauseMutation.isPending ? "Pausing..." : "Pause"}
                  </Button>
                )}
                <Button variant="destructive" size="sm" onClick={() => setDeleteConfirmOpen(true)} disabled={deleteMutation.isPending}>
                  {deleteMutation.isPending ? "Deleting..." : "Delete"}
                </Button>
              </>
            )}
          </div>
        }
      />

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <StatCard label="State" value={connector.state.toUpperCase()} icon={Activity} accent={connector.state === "RUNNING" ? "success" : connector.state === "FAILED" ? "destructive" : "warning"} />
        <StatCard label="Type" value={connector.type} icon={PlugZap} accent="primary" />
        <StatCard label="Worker" value={connector.workerId} icon={Server} accent="primary" />
        <StatCard label="Tasks" value={connector.tasks.length} icon={ListTodo} accent="primary" />
      </div>

      {/* Tasks Table */}
      <Card className="animate-scale-in">
        <CardHeader>
          <CardTitle>Tasks</CardTitle>
        </CardHeader>
        <CardContent>
          {connector.tasks.length === 0 ? (
            <EmptyState icon={ListTodo} title="No tasks" description="This connector has no running tasks." />
          ) : (
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>Task ID</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead>Worker ID</TableHead>
                  <TableHead>Trace</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {connector.tasks.map((task, i) => (
                  <TableRow key={task.id} className={rowClassName(i)}>
                    <TableCell>{task.id}</TableCell>
                    <TableCell>
                      <Badge variant={getConnectorStateBadgeVariant(task.state)}>{task.state.toUpperCase()}</Badge>
                    </TableCell>
                    <TableCell className="font-mono text-sm">{task.workerId}</TableCell>
                    <TableCell className="text-sm text-muted-foreground max-w-md truncate">
                      {task.trace ?? "-"}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Configuration Section */}
      <Card className="animate-scale-in">
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Configuration</CardTitle>
            {canManageConnectors && (
            <Dialog open={editDialogOpen} onOpenChange={setEditDialogOpen}>
              <DialogTrigger asChild>
                <Button variant="outline" size="sm" onClick={openEditDialog}>
                  Edit
                </Button>
              </DialogTrigger>
              <DialogContent className="max-w-2xl">
                <DialogHeader>
                  <DialogTitle>Edit Connector Configuration</DialogTitle>
                </DialogHeader>
                <div className="space-y-4">
                  <Textarea
                    value={configJson}
                    onChange={(e) => setConfigJson(e.target.value)}
                    className="font-mono text-sm min-h-[300px]"
                  />
                  {(configError || updateConfigMutation.isError) && (
                    <p className="text-sm text-destructive">
                      {configError ?? getErrorMessage(updateConfigMutation.error)}
                    </p>
                  )}
                  <div className="flex justify-end gap-2">
                    <Button variant="outline" onClick={() => setEditDialogOpen(false)}>
                      Cancel
                    </Button>
                    <Button onClick={handleSaveConfig} disabled={updateConfigMutation.isPending}>
                      {updateConfigMutation.isPending ? "Saving..." : "Save"}
                    </Button>
                  </div>
                </div>
              </DialogContent>
            </Dialog>
            )}
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>Key</TableHead>
                <TableHead>Value</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedConfigKeys.map((key, i) => (
                <TableRow key={key} className={rowClassName(i)}>
                  <TableCell className="font-mono text-sm">{key}</TableCell>
                  <TableCell className="font-mono text-sm break-all">{connector.config[key]}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete Connector"
        description={`Are you sure you want to delete connector "${connectorName}"? This action cannot be undone.`}
        onConfirm={() => deleteMutation.mutate()}
        destructive
      />
    </div>
  );
}
