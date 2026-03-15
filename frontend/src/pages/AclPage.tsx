import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { useHasAction } from "@/hooks/usePermissions";
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
import { Shield, Plus } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { getErrorMessage } from "@/lib/error-utils";

interface ACLEntry {
  resourceType: string;
  resourceName: string;
  patternType: string;
  principal: string;
  host: string;
  operation: string;
  permission: string;
}

const RESOURCE_TYPES = ["TOPIC", "GROUP", "CLUSTER", "TRANSACTIONAL_ID"] as const;
const PATTERN_TYPES = ["LITERAL", "PREFIXED"] as const;
const OPERATIONS = [
  "ALL", "READ", "WRITE", "CREATE", "DELETE", "ALTER", "DESCRIBE",
  "CLUSTER_ACTION", "DESCRIBE_CONFIGS", "ALTER_CONFIGS", "IDEMPOTENT_WRITE",
] as const;
const PERMISSIONS = ["ALLOW", "DENY"] as const;

const initialForm = {
  resourceType: "TOPIC" as string,
  resourceName: "",
  patternType: "LITERAL" as string,
  principal: "",
  host: "*",
  operation: "ALL" as string,
  permission: "ALLOW" as string,
};

export function AclPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const queryClient = useQueryClient();
  const canCreateAcls = useHasAction("create_acls");
  const canDeleteAcls = useHasAction("delete_acls");
  const [search, setSearch] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ACLEntry | null>(null);
  const [form, setForm] = useState(initialForm);

  const { data: acls, isLoading, error, refetch } = useQuery({
    queryKey: ["acls", clusterName],
    queryFn: () => api.acl.list(clusterName!),
    enabled: !!clusterName,
  });

  const createMutation = useMutation({
    mutationFn: (entry: ACLEntry) => api.acl.create(clusterName!, entry),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["acls", clusterName] });
      setDialogOpen(false);
      setForm(initialForm);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (entry: ACLEntry) => api.acl.delete(clusterName!, entry),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["acls", clusterName] });
    },
  });

  const handleCreate = () => {
    const entry: ACLEntry = {
      resourceType: form.resourceType,
      resourceName: form.resourceType === "CLUSTER" ? "" : form.resourceName,
      patternType: form.patternType,
      principal: form.principal,
      host: form.host,
      operation: form.operation,
      permission: form.permission,
    };
    createMutation.mutate(entry);
  };

  const handleDelete = (entry: ACLEntry) => {
    setDeleteTarget(entry);
  };

  const filteredAcls = acls?.filter((acl: ACLEntry) => {
    const term = search.toLowerCase();
    return (
      acl.principal.toLowerCase().includes(term) ||
      acl.resourceName.toLowerCase().includes(term) ||
      acl.operation.toLowerCase().includes(term)
    );
  }) ?? [];

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "ACL Management" },
  ];

  if (isLoading) return <><PageHeader title="ACL Management" breadcrumbs={breadcrumbs} /><TableSkeleton cols={8} /></>;
  if (error) {
    const msg = getErrorMessage(error);
    const notConfigured = msg.toLowerCase().includes("not configured") || msg.toLowerCase().includes("no authorizer");
    return (
      <div>
        <PageHeader title="ACL Management" breadcrumbs={breadcrumbs} />
        {notConfigured ? (
          <EmptyState icon={Shield} title="ACL Management not available" description="No Authorizer is configured on the broker. Enable an authorizer in your Kafka broker configuration to manage access control lists." />
        ) : (
          <ErrorAlert error={error} onRetry={() => refetch()} />
        )}
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title="ACL Management"
        description={`Access control lists for ${clusterName}`}
        breadcrumbs={breadcrumbs}
        actions={canCreateAcls ? (
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button><Plus className="h-4 w-4 mr-2" />Create ACL</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create ACL</DialogTitle>
              </DialogHeader>
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label>Resource Type</Label>
                  <Select value={form.resourceType} onValueChange={(v) => setForm({ ...form, resourceType: v })}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {RESOURCE_TYPES.map((rt) => (
                        <SelectItem key={rt} value={rt}>{rt}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                {form.resourceType !== "CLUSTER" && (
                  <div className="space-y-2">
                    <Label>Resource Name</Label>
                    <Input
                      value={form.resourceName}
                      onChange={(e) => setForm({ ...form, resourceName: e.target.value })}
                      placeholder="Resource name"
                    />
                  </div>
                )}
                <div className="space-y-2">
                  <Label>Pattern Type</Label>
                  <Select value={form.patternType} onValueChange={(v) => setForm({ ...form, patternType: v })}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {PATTERN_TYPES.map((pt) => (
                        <SelectItem key={pt} value={pt}>{pt}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>Principal</Label>
                  <Input
                    value={form.principal}
                    onChange={(e) => setForm({ ...form, principal: e.target.value })}
                    placeholder="User:alice"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Host</Label>
                  <Input
                    value={form.host}
                    onChange={(e) => setForm({ ...form, host: e.target.value })}
                    placeholder="*"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Operation</Label>
                  <Select value={form.operation} onValueChange={(v) => setForm({ ...form, operation: v })}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {OPERATIONS.map((op) => (
                        <SelectItem key={op} value={op}>{op}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>Permission</Label>
                  <Select value={form.permission} onValueChange={(v) => setForm({ ...form, permission: v })}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {PERMISSIONS.map((p) => (
                        <SelectItem key={p} value={p}>{p}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <Button onClick={handleCreate} disabled={createMutation.isPending} className="w-full">
                  {createMutation.isPending ? "Creating..." : "Create"}
                </Button>
                {createMutation.isError && (
                  <ErrorAlert error={createMutation.error} />
                )}
              </div>
            </DialogContent>
          </Dialog>
        ) : undefined}
      />
      <div className="mb-4">
        <Input
          placeholder="Search by principal, resource name, or operation..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>
      {filteredAcls.length === 0 ? (
        <EmptyState icon={Shield} title="No ACLs found" description={search ? "No ACLs match your search." : "No access control lists configured yet."} actionLabel={!search && canCreateAcls ? "Create ACL" : undefined} onAction={!search && canCreateAcls ? () => setDialogOpen(true) : undefined} />
      ) : (
        <DataTable<ACLEntry>
          itemName="ACL entries"
          data={filteredAcls}
          columns={[
            {
              header: "Resource Type",
              cell: (acl) => {
                const variant = acl.resourceType === "TOPIC" ? "default" as const : acl.resourceType === "GROUP" ? "secondary" as const : "outline" as const;
                return <Badge variant={variant}>{acl.resourceType}</Badge>;
              },
            },
            { header: "Resource Name", accessorKey: "resourceName" },
            { header: "Pattern Type", accessorKey: "patternType" },
            { header: "Principal", accessorKey: "principal" },
            { header: "Host", accessorKey: "host" },
            {
              header: "Operation",
              cell: (acl) => <Badge variant="secondary">{acl.operation}</Badge>,
            },
            {
              header: "Permission",
              cell: (acl) => <Badge variant={acl.permission === "DENY" ? "destructive" : "success"}>{acl.permission}</Badge>,
            },
            {
              header: "Actions",
              cell: (acl) => canDeleteAcls ? (
                <Button variant="destructive" size="sm" onClick={() => handleDelete(acl)} disabled={deleteMutation.isPending}>
                  Delete
                </Button>
              ) : null,
            },
          ]}
        />
      )}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}
        title="Delete ACL Entry"
        description={`Are you sure you want to delete this ACL entry for principal "${deleteTarget?.principal}" on ${deleteTarget?.resourceType} "${deleteTarget?.resourceName}"?`}
        onConfirm={() => { if (deleteTarget) { deleteMutation.mutate(deleteTarget); setDeleteTarget(null); } }}
        destructive
      />
    </div>
  );
}
