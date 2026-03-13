import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { ErrorAlert } from "@/components/ErrorAlert";
import { TopicTabs } from "@/components/TopicTabs";
import { PageHeader } from "@/components/PageHeader";
import { DetailSkeleton } from "@/components/PageSkeleton";
import { rowClassName } from "@/lib/utils";

export function TopicDetailPage() {
  const { clusterName, topicName } = useParams<{ clusterName: string; topicName: string }>();
  const { data: topic, isLoading, error, refetch } = useQuery({
    queryKey: ["topic", clusterName, topicName],
    queryFn: () => api.topics.details(clusterName!, topicName!),
    enabled: !!clusterName && !!topicName,
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
        actions={topic.internal ? <Badge variant="secondary">internal</Badge> : undefined}
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
    </div>
  );
}
