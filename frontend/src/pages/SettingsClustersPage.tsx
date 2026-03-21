import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type AdminClusterInfo } from "@/lib/api";
import { useHasAction } from "@/hooks/usePermissions";
import { PageHeader } from "@/components/PageHeader";
import { ErrorAlert } from "@/components/ErrorAlert";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Plus, Pencil, Trash2, Database } from "lucide-react";
import { getErrorMessage } from "@/lib/error-utils";
import { ClusterWizard } from "@/components/wizard/ClusterWizard";

export function SettingsClustersPage() {
  const queryClient = useQueryClient();
  const canManageClusters = useHasAction("manage_clusters");
  const [wizardOpen, setWizardOpen] = useState(false);
  const [editCluster, setEditCluster] = useState<AdminClusterInfo | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["admin-clusters"],
    queryFn: api.admin.listClusters,
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) => api.admin.deleteCluster(name),
    onSuccess: () => {
      setDeleteConfirm(null);
      queryClient.invalidateQueries({ queryKey: ["admin-clusters"] });
      queryClient.invalidateQueries({ queryKey: ["clusters"] });
    },
  });

  const handleSaved = () => {
    setWizardOpen(false);
    setEditCluster(null);
    queryClient.invalidateQueries({ queryKey: ["admin-clusters"] });
    queryClient.invalidateQueries({ queryKey: ["clusters"] });
  };

  const allClusters = [
    ...(data?.static.map((c) => ({ ...c, source: "static" as const })) || []),
    ...(data?.dynamic.map((c) => ({ ...c, source: "dynamic" as const })) || []),
  ];

  if (!canManageClusters) {
    return (
      <div className="p-8 text-center text-muted-foreground">Access denied.</div>
    );
  }

  return (
    <div>
      <PageHeader
        title="Cluster Settings"
        description="Manage Kafka cluster connections"
        breadcrumbs={[{ label: "Dashboard", href: "/" }, { label: "Settings" }]}
        actions={<Button onClick={() => setWizardOpen(true)}><Plus className="h-4 w-4 mr-2" />Add Cluster</Button>}
      />

      {isLoading && <TableSkeleton rows={3} cols={3} />}
      {error && <ErrorAlert error={error} onRetry={() => refetch()} />}

      {!isLoading && !error && allClusters.length === 0 && (
        <EmptyState
          icon={Database}
          title="No clusters"
          description="Add a Kafka cluster to get started."
          actionLabel="Add Cluster"
          onAction={() => setWizardOpen(true)}
        />
      )}

      {allClusters.length > 0 && (
        <div className="rounded-lg border bg-card animate-scale-in">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Bootstrap Servers</TableHead>
                <TableHead>Source</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {allClusters.map((c) => (
                <TableRow key={c.name}>
                  <TableCell className="font-medium">{c.name}</TableCell>
                  <TableCell className="font-mono text-sm">{c.bootstrapServers}</TableCell>
                  <TableCell>
                    <Badge variant={c.source === "static" ? "secondary" : "outline"}>
                      {c.source}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {c.source === "dynamic" && (
                      <div className="flex gap-1">
                        <Button variant="ghost" size="icon" onClick={() => setEditCluster(c)}>
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => setDeleteConfirm(c.name)}>
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Cluster Wizard — key forces remount to reset state */}
      <ClusterWizard
        key={wizardOpen ? "add" : "closed"}
        open={wizardOpen}
        onClose={() => setWizardOpen(false)}
        onSaved={handleSaved}
      />

      {/* Edit Wizard — note: only name+bootstrapServers available from list endpoint */}
      {editCluster && (
        <ClusterWizard
          open
          onClose={() => setEditCluster(null)}
          onSaved={handleSaved}
          initialData={editCluster}
        />
      )}

      {/* Delete Confirmation Dialog */}
      <Dialog open={!!deleteConfirm} onOpenChange={() => setDeleteConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Cluster</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Are you sure you want to delete <span className="font-medium text-foreground">{deleteConfirm}</span>? This action cannot be undone.
          </p>
          {deleteMutation.error && <p className="text-sm text-destructive">{getErrorMessage(deleteMutation.error)}</p>}
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteConfirm(null)}>Cancel</Button>
            <Button variant="destructive" onClick={() => deleteConfirm && deleteMutation.mutate(deleteConfirm)} disabled={deleteMutation.isPending}>
              {deleteMutation.isPending ? "Deleting..." : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
