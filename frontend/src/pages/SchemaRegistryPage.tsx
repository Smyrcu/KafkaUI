import { useState, useCallback } from "react";
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { PageHeader } from "@/components/PageHeader";
import { DataTable } from "@/components/DataTable";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";
import { useSearchFilter } from "@/hooks/useSearchFilter";
import { getErrorMessage } from "@/lib/error-utils";
import { BookOpen, Plus } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import type { SchemaSubjectInfo } from "@/lib/api";

export function SchemaRegistryPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const queryClient = useQueryClient();
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const schemaAccessor = useCallback((s: SchemaSubjectInfo) => s.subject, []);
  const [newSubject, setNewSubject] = useState("");
  const [newSchemaType, setNewSchemaType] = useState("AVRO");
  const [newSchema, setNewSchema] = useState("");

  const { data: schemas, isLoading, error, refetch } = useQuery({
    queryKey: ["schemas", clusterName],
    queryFn: () => api.schemas.list(clusterName!),
    enabled: !!clusterName,
  });

  const createMutation = useMutation({
    mutationFn: () =>
      api.schemas.create(clusterName!, {
        subject: newSubject,
        schema: newSchema,
        schemaType: newSchemaType,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["schemas", clusterName] });
      setCreateOpen(false);
      setNewSubject("");
      setNewSchemaType("AVRO");
      setNewSchema("");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (subject: string) => api.schemas.delete(clusterName!, subject),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["schemas", clusterName] });
    },
  });

  const { search, setSearch, filtered: filteredSchemas } = useSearchFilter(schemas ?? [], schemaAccessor);

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Schema Registry" },
  ];

  if (isLoading) return <><PageHeader title="Schema Registry" breadcrumbs={breadcrumbs} /><TableSkeleton cols={5} /></>;
  if (error) {
    const msg = getErrorMessage(error);
    const notConfigured = msg.toLowerCase().includes("not configured");
    return (
      <div>
        <PageHeader title="Schema Registry" breadcrumbs={breadcrumbs} />
        {notConfigured ? (
          <EmptyState icon={BookOpen} title="Schema Registry not configured" description="No Schema Registry URL is configured for this cluster. Add a schemaRegistry.url to your cluster configuration to manage schemas." />
        ) : (
          <ErrorAlert error={error} onRetry={() => refetch()} />
        )}
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title="Schema Registry"
        description={`Manage schemas in ${clusterName}`}
        breadcrumbs={breadcrumbs}
        actions={
          <Dialog open={createOpen} onOpenChange={setCreateOpen}>
            <DialogTrigger asChild>
              <Button><Plus className="h-4 w-4 mr-2" />Create Schema</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create Schema</DialogTitle>
              </DialogHeader>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  createMutation.mutate();
                }}
                className="space-y-4"
              >
                <div className="space-y-2">
                  <Label htmlFor="subject">Subject</Label>
                  <Input
                    id="subject"
                    placeholder="e.g. my-topic-value"
                    value={newSubject}
                    onChange={(e) => setNewSubject(e.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="schemaType">Schema Type</Label>
                  <Select value={newSchemaType} onValueChange={setNewSchemaType}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select schema type" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="AVRO">AVRO</SelectItem>
                      <SelectItem value="JSON">JSON</SelectItem>
                      <SelectItem value="PROTOBUF">PROTOBUF</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="schema">Schema</Label>
                  <Textarea
                    id="schema"
                    placeholder="Paste your schema here..."
                    value={newSchema}
                    onChange={(e) => setNewSchema(e.target.value)}
                    rows={10}
                    required
                  />
                </div>
                <Button type="submit" disabled={createMutation.isPending}>
                  {createMutation.isPending ? "Creating..." : "Create"}
                </Button>
                {createMutation.isError && (
                  <ErrorAlert error={createMutation.error} />
                )}
              </form>
            </DialogContent>
          </Dialog>
        }
      />
      <div className="mb-4">
        <Input
          placeholder="Search schemas..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>
      {filteredSchemas.length === 0 ? (
        <EmptyState icon={BookOpen} title="No schemas found" description={search ? "No schemas match your search." : "No schemas registered yet."} actionLabel={!search ? "Create Schema" : undefined} onAction={!search ? () => setCreateOpen(true) : undefined} />
      ) : (
        <DataTable<SchemaSubjectInfo>
          itemName="schemas"
          data={filteredSchemas}
          columns={[
            {
              header: "Subject",
              cell: (s) => (
                <Link to={`/clusters/${clusterName}/schemas/${encodeURIComponent(s.subject)}`} className="text-primary hover:underline font-medium">
                  {s.subject}
                </Link>
              ),
            },
            { header: "Latest Version", accessorKey: "latestVersion" },
            { header: "Schema ID", accessorKey: "latestSchemaId" },
            {
              header: "Type",
              cell: (s) => <Badge variant={s.schemaType === "AVRO" ? "default" : s.schemaType === "JSON" ? "secondary" : "outline"}>{s.schemaType}</Badge>,
            },
            {
              header: "Actions",
              cell: (s) => (
                <Button
                  variant="destructive"
                  size="sm"
                  disabled={deleteMutation.isPending}
                  onClick={() => setDeleteTarget(s.subject)}
                >
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
        title="Delete Schema"
        description={`Are you sure you want to delete schema "${deleteTarget}"? This action cannot be undone.`}
        onConfirm={() => { if (deleteTarget) deleteMutation.mutate(deleteTarget); }}
        destructive
      />
    </div>
  );
}
