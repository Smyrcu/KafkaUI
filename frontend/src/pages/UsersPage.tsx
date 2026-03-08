import { useState, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";
import { DataTable } from "@/components/DataTable";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";
import { useSearchFilter } from "@/hooks/useSearchFilter";
import { UserCog, Plus } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";

interface ScramUser {
  name: string;
  mechanism: string;
  iterations: number;
}

const MECHANISMS = ["SCRAM-SHA-256", "SCRAM-SHA-512"] as const;

const initialForm = {
  name: "",
  password: "",
  mechanism: "SCRAM-SHA-256" as string,
  iterations: "4096",
};

export function UsersPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const queryClient = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ScramUser | null>(null);
  const userAccessor = useCallback((user: ScramUser) => user.name, []);
  const [form, setForm] = useState(initialForm);

  const { data: users, isLoading, error, refetch } = useQuery({
    queryKey: ["users", clusterName],
    queryFn: () => api.users.list(clusterName!),
    enabled: !!clusterName,
  });

  const createMutation = useMutation({
    mutationFn: (data: { name: string; password: string; mechanism: string; iterations?: number }) =>
      api.users.create(clusterName!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", clusterName] });
      setDialogOpen(false);
      setForm(initialForm);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (data: { name: string; mechanism: string }) =>
      api.users.delete(clusterName!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users", clusterName] });
    },
  });

  const handleCreate = () => {
    const iterations = parseInt(form.iterations, 10);
    createMutation.mutate({
      name: form.name,
      password: form.password,
      mechanism: form.mechanism,
      iterations: isNaN(iterations) ? 4096 : iterations,
    });
  };

  const handleDelete = (user: ScramUser) => {
    setDeleteTarget(user);
  };

  const { search, setSearch, filtered: filteredUsers } = useSearchFilter(users ?? [] as ScramUser[], userAccessor);

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Users" },
  ];

  if (isLoading) return <><PageHeader title="Users" breadcrumbs={breadcrumbs} /><TableSkeleton cols={4} /></>;
  if (error) {
    return (
      <div>
        <PageHeader title="Users" breadcrumbs={breadcrumbs} />
        <ErrorAlert error={error} onRetry={() => refetch()} />
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title="Users"
        description={`SCRAM credentials for ${clusterName}`}
        breadcrumbs={breadcrumbs}
        actions={
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button><Plus className="h-4 w-4 mr-2" />Create User</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create SCRAM User</DialogTitle>
              </DialogHeader>
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label>Username</Label>
                  <Input
                    value={form.name}
                    onChange={(e) => setForm({ ...form, name: e.target.value })}
                    placeholder="Username"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Password</Label>
                  <Input
                    type="password"
                    value={form.password}
                    onChange={(e) => setForm({ ...form, password: e.target.value })}
                    placeholder="Password"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Mechanism</Label>
                  <Select value={form.mechanism} onValueChange={(v) => setForm({ ...form, mechanism: v })}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {MECHANISMS.map((m) => (
                        <SelectItem key={m} value={m}>{m}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>Iterations</Label>
                  <Input
                    type="number"
                    value={form.iterations}
                    onChange={(e) => setForm({ ...form, iterations: e.target.value })}
                    placeholder="4096"
                  />
                </div>
                <Button onClick={handleCreate} disabled={createMutation.isPending || !form.name || !form.password} className="w-full">
                  {createMutation.isPending ? "Creating..." : "Create"}
                </Button>
                {createMutation.isError && (
                  <ErrorAlert error={createMutation.error} />
                )}
              </div>
            </DialogContent>
          </Dialog>
        }
      />
      <div className="mb-4">
        <Input
          placeholder="Search by username..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>
      {filteredUsers.length === 0 ? (
        <EmptyState icon={UserCog} title="No SCRAM users found" description={search ? "No users match your search." : "No SCRAM credentials configured yet."} actionLabel={!search ? "Create User" : undefined} onAction={!search ? () => setDialogOpen(true) : undefined} />
      ) : (
        <DataTable<ScramUser>
          itemName="users"
          data={filteredUsers}
          columns={[
            { header: "Username", accessorKey: "name" },
            {
              header: "Mechanism",
              cell: (user) => <Badge variant="secondary">{user.mechanism}</Badge>,
            },
            { header: "Iterations", accessorKey: "iterations" },
            {
              header: "Actions",
              cell: (user) => (
                <Button variant="destructive" size="sm" onClick={() => handleDelete(user)} disabled={deleteMutation.isPending}>
                  Delete
                </Button>
              ),
            },
          ]}
        />
      )}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}
        title="Delete SCRAM Credentials"
        description={`Are you sure you want to delete SCRAM credentials for "${deleteTarget?.name}" (${deleteTarget?.mechanism})? This action cannot be undone.`}
        onConfirm={() => { if (deleteTarget) deleteMutation.mutate({ name: deleteTarget.name, mechanism: deleteTarget.mechanism }); setDeleteTarget(null); }}
        destructive
      />
    </div>
  );
}
