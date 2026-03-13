import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type AddClusterRequest, type AdminClusterInfo } from "@/lib/api";
import { PageHeader } from "@/components/PageHeader";
import { ErrorAlert } from "@/components/ErrorAlert";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Checkbox } from "@/components/ui/checkbox";
import { Plus, Pencil, Trash2, Database, Plug, Loader2 } from "lucide-react";
import { getErrorMessage } from "@/lib/error-utils";

const emptyForm: AddClusterRequest = {
  name: "",
  bootstrapServers: "",
};

export function SettingsClustersPage() {
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editName, setEditName] = useState<string | null>(null);
  const [form, setForm] = useState<AddClusterRequest>(emptyForm);
  const [skipValidation, setSkipValidation] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["admin-clusters"],
    queryFn: api.admin.listClusters,
  });

  const addMutation = useMutation({
    mutationFn: (data: AddClusterRequest) => api.admin.addCluster(data, !skipValidation),
    onSuccess: () => {
      setDialogOpen(false);
      setForm(emptyForm);
      queryClient.invalidateQueries({ queryKey: ["admin-clusters"] });
      queryClient.invalidateQueries({ queryKey: ["clusters"] });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ name, data }: { name: string; data: AddClusterRequest }) =>
      api.admin.updateCluster(name, data, !skipValidation),
    onSuccess: () => {
      setDialogOpen(false);
      setEditName(null);
      setForm(emptyForm);
      queryClient.invalidateQueries({ queryKey: ["admin-clusters"] });
      queryClient.invalidateQueries({ queryKey: ["clusters"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) => api.admin.deleteCluster(name),
    onSuccess: () => {
      setDeleteConfirm(null);
      queryClient.invalidateQueries({ queryKey: ["admin-clusters"] });
      queryClient.invalidateQueries({ queryKey: ["clusters"] });
    },
  });

  const testMutation = useMutation({
    mutationFn: (data: AddClusterRequest) => api.admin.testConnection(data),
  });

  const openAdd = () => {
    setEditName(null);
    setForm(emptyForm);
    setSkipValidation(false);
    testMutation.reset();
    setDialogOpen(true);
  };

  const openEdit = (cluster: AdminClusterInfo) => {
    setEditName(cluster.name);
    setForm({ name: cluster.name, bootstrapServers: cluster.bootstrapServers });
    setSkipValidation(false);
    testMutation.reset();
    setDialogOpen(true);
  };

  const handleSubmit = () => {
    if (editName) {
      updateMutation.mutate({ name: editName, data: form });
    } else {
      addMutation.mutate(form);
    }
  };

  const isPending = addMutation.isPending || updateMutation.isPending;
  const mutationError = addMutation.error || updateMutation.error;
  const allClusters = [
    ...(data?.static.map((c) => ({ ...c, source: "static" as const })) || []),
    ...(data?.dynamic.map((c) => ({ ...c, source: "dynamic" as const })) || []),
  ];

  return (
    <div>
      <PageHeader
        title="Cluster Settings"
        description="Manage Kafka cluster connections"
        breadcrumbs={[{ label: "Dashboard", href: "/" }, { label: "Settings" }]}
        actions={<Button onClick={openAdd}><Plus className="h-4 w-4 mr-2" />Add Cluster</Button>}
      />

      {isLoading && <TableSkeleton rows={3} cols={3} />}
      {error && <ErrorAlert error={error} onRetry={() => refetch()} />}

      {!isLoading && !error && allClusters.length === 0 && (
        <EmptyState
          icon={Database}
          title="No clusters"
          description="Add a Kafka cluster to get started."
          actionLabel="Add Cluster"
          onAction={openAdd}
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
                        <Button variant="ghost" size="icon" onClick={() => openEdit(c)}>
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

      {/* Add/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editName ? "Edit Cluster" : "Add Cluster"}</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label>Name</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="my-cluster"
                disabled={!!editName}
              />
            </div>
            <div className="grid gap-2">
              <Label>Bootstrap Servers</Label>
              <Input
                value={form.bootstrapServers}
                onChange={(e) => setForm({ ...form, bootstrapServers: e.target.value })}
                placeholder="localhost:9092"
              />
            </div>
            <div className="flex items-center gap-2">
              <Checkbox
                id="skip-validation"
                checked={skipValidation}
                onCheckedChange={(v) => setSkipValidation(v === true)}
              />
              <Label htmlFor="skip-validation" className="text-sm font-normal">Save without testing connection</Label>
            </div>
            <Button
              variant="outline"
              onClick={() => testMutation.mutate(form)}
              disabled={testMutation.isPending || !form.bootstrapServers}
            >
              {testMutation.isPending ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : <Plug className="h-4 w-4 mr-2" />}
              Test Connection
            </Button>
            {testMutation.data && (
              <p className={`text-sm ${testMutation.data.status === "ok" ? "text-green-600" : "text-destructive"}`}>
                {testMutation.data.status === "ok" ? "Connection successful" : `Failed: ${testMutation.data.error}`}
              </p>
            )}
            {mutationError && <p className="text-sm text-destructive">{getErrorMessage(mutationError)}</p>}
          </div>
          <DialogFooter>
            <Button onClick={handleSubmit} disabled={isPending || !form.name || !form.bootstrapServers}>
              {isPending ? "Saving..." : editName ? "Update" : "Add"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

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
