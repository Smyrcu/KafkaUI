import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, useNavigate } from "react-router-dom";
import { api } from "@/lib/api";
import { useHasAction } from "@/hooks/usePermissions";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ErrorAlert } from "@/components/ErrorAlert";
import { TopicTabs } from "@/components/TopicTabs";
import { PageHeader } from "@/components/PageHeader";
import { DetailSkeleton } from "@/components/PageSkeleton";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { Trash2 } from "lucide-react";
import { rowClassName } from "@/lib/utils";

export function TopicDetailPage() {
  const { clusterName, topicName } = useParams<{ clusterName: string; topicName: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const canDeleteTopics = useHasAction("delete_topics");
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  const { data: topic, isLoading, error, refetch } = useQuery({
    queryKey: ["topic", clusterName, topicName],
    queryFn: () => api.topics.details(clusterName!, topicName!),
    enabled: !!clusterName && !!topicName,
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.topics.delete(clusterName!, topicName!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["topics", clusterName] });
      navigate(`/clusters/${clusterName}/topics`);
    },
  });

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Topics", href: `/clusters/${clusterName}/topics` },
    { label: topicName! },
  ];

  if (isLoading) return <><PageHeader title={topicName!} breadcrumbs={breadcrumbs} /><DetailSkeleton /></>;
  if (error) return <><PageHeader title={topicName!} breadcrumbs={breadcrumbs} /><ErrorAlert error={error} onRetry={() => refetch()} /></>;
  if (!topic) return null;

  return (
    <div className="space-y-6">
      <PageHeader
        title={topic.name}
        breadcrumbs={breadcrumbs}
        actions={
          <div className="flex items-center gap-2">
            {topic.internal && <Badge variant="secondary">internal</Badge>}
            {canDeleteTopics && !topic.internal && (
              <Button variant="destructive" size="sm" onClick={() => setDeleteConfirmOpen(true)} disabled={deleteMutation.isPending}>
                <Trash2 className="h-4 w-4 mr-2" />
                {deleteMutation.isPending ? "Deleting..." : "Delete Topic"}
              </Button>
            )}
          </div>
        }
      />
      <TopicTabs />
      <div className="grid gap-4 md:grid-cols-2">
        <Card className="animate-scale-in">
          <CardHeader><CardTitle>Partitions</CardTitle></CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Partition</TableHead><TableHead>Leader</TableHead><TableHead>Replicas</TableHead><TableHead>ISR</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {topic.partitions.map((p) => (
                  <TableRow key={p.id} className={rowClassName(p.id)}>
                    <TableCell><Badge variant="outline">{p.id}</Badge></TableCell>
                    <TableCell>{p.leader}</TableCell>
                    <TableCell>{p.replicas.join(", ")}</TableCell>
                    <TableCell>{p.isr.join(", ")}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
        <Card className="animate-scale-in">
          <CardHeader><CardTitle>Configuration</CardTitle></CardHeader>
          <CardContent>
            <Table>
              <TableHeader><TableRow><TableHead>Key</TableHead><TableHead>Value</TableHead></TableRow></TableHeader>
              <TableBody>
                {Object.entries(topic.configs).map(([key, value], i) => (
                  <TableRow key={key} className={rowClassName(i)}>
                    <TableCell className="font-mono text-xs">{key}</TableCell>
                    <TableCell className="font-mono text-xs">{value}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete Topic"
        description={`Are you sure you want to delete topic "${topicName}"? This action cannot be undone.`}
        onConfirm={() => deleteMutation.mutate()}
        destructive
      />
    </div>
  );
}
