import { useState, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link } from "react-router-dom";
import { api, type CreateTopicRequest } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger, DialogFooter } from "@/components/ui/dialog";
import { Plus, Trash2, FileText } from "lucide-react";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";
import { DataTable } from "@/components/DataTable";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { useSearchFilter } from "@/hooks/useSearchFilter";
import { getErrorMessage } from "@/lib/error-utils";
import type { TopicInfo } from "@/lib/api";

export function TopicsPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [newTopic, setNewTopic] = useState<CreateTopicRequest>({ name: "", partitions: 1, replicas: 1 });
  const topicAccessor = useCallback((t: TopicInfo) => t.name, []);

  const { data: topics, isLoading, error, refetch } = useQuery({
    queryKey: ["topics", clusterName],
    queryFn: () => api.topics.list(clusterName!),
    enabled: !!clusterName,
  });

  const createMutation = useMutation({
    mutationFn: (data: CreateTopicRequest) => api.topics.create(clusterName!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["topics", clusterName] });
      setOpen(false);
      setNewTopic({ name: "", partitions: 1, replicas: 1 });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (topicName: string) => api.topics.delete(clusterName!, topicName),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["topics", clusterName] }); },
  });

  const { search, setSearch, filtered: filteredTopics } = useSearchFilter(topics ?? [], topicAccessor);

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Topics" },
  ];

  if (isLoading) return <><PageHeader title="Topics" breadcrumbs={breadcrumbs} /><TableSkeleton cols={5} /></>;
  if (error) return <ErrorAlert error={error} onRetry={() => refetch()} />;

  return (
    <div>
      <PageHeader
        title="Topics"
        description={`Manage topics in ${clusterName}`}
        breadcrumbs={breadcrumbs}
        actions={
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button><Plus className="h-4 w-4 mr-2" />Create Topic</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader><DialogTitle>Create Topic</DialogTitle></DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="grid gap-2">
                  <Label htmlFor="name">Name</Label>
                  <Input id="name" value={newTopic.name} onChange={(e) => setNewTopic({ ...newTopic, name: e.target.value })} placeholder="my-topic" />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="grid gap-2">
                    <Label htmlFor="partitions">Partitions</Label>
                    <Input id="partitions" type="number" min={1} value={newTopic.partitions} onChange={(e) => setNewTopic({ ...newTopic, partitions: parseInt(e.target.value) || 1 })} />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="replicas">Replicas</Label>
                    <Input id="replicas" type="number" min={1} value={newTopic.replicas} onChange={(e) => setNewTopic({ ...newTopic, replicas: parseInt(e.target.value) || 1 })} />
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button onClick={() => createMutation.mutate(newTopic)} disabled={!newTopic.name || createMutation.isPending}>
                  {createMutation.isPending ? "Creating..." : "Create"}
                </Button>
              </DialogFooter>
              {createMutation.isError && <p className="text-sm text-destructive mt-2">{getErrorMessage(createMutation.error)}</p>}
            </DialogContent>
          </Dialog>
        }
      />
      <div className="mb-4">
        <Input placeholder="Search topics..." value={search} onChange={(e) => setSearch(e.target.value)} className="max-w-sm" />
      </div>
      {filteredTopics.length === 0 ? (
        <EmptyState icon={FileText} title="No topics found" description={search ? "No topics match your search." : "This cluster has no topics yet."} actionLabel={!search ? "Create Topic" : undefined} onAction={!search ? () => setOpen(true) : undefined} />
      ) : (
        <DataTable<TopicInfo>
          itemName="topics"
          data={filteredTopics}
          columns={[
            { header: "Name", cell: (t) => <Link to={`/clusters/${clusterName}/topics/${t.name}`} className="text-primary hover:underline font-medium">{t.name}</Link> },
            { header: "Partitions", accessorKey: "partitions" },
            { header: "Replicas", accessorKey: "replicas" },
            { header: "Internal", cell: (t) => t.internal ? <Badge variant="secondary">internal</Badge> : null },
            { header: "Actions", className: "w-[80px]", cell: (t) => !t.internal ? (
              <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteTarget(t.name); }}>
                <Trash2 className="h-4 w-4 text-destructive" />
              </Button>
            ) : null },
          ]}
        />
      )}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}
        title="Delete Topic"
        description={`Are you sure you want to delete topic "${deleteTarget}"? This action cannot be undone.`}
        onConfirm={() => { if (deleteTarget) deleteMutation.mutate(deleteTarget, { onSuccess: () => setDeleteTarget(null) }); }}
        destructive
      />
    </div>
  );
}
