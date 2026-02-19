import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";

export function TopicDetailPage() {
  const { clusterName, topicName } = useParams<{ clusterName: string; topicName: string }>();
  const { data: topic, isLoading, error } = useQuery({
    queryKey: ["topic", clusterName, topicName],
    queryFn: () => api.topics.details(clusterName!, topicName!),
    enabled: !!clusterName && !!topicName,
  });
  if (isLoading) return <div className="text-muted-foreground">Loading topic details...</div>;
  if (error) return <div className="text-destructive">Error: {(error as Error).message}</div>;
  if (!topic) return null;
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <h2 className="text-2xl font-bold">{topic.name}</h2>
        {topic.internal && <Badge variant="secondary">internal</Badge>}
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
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
                  <TableRow key={p.id}>
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
        <Card>
          <CardHeader><CardTitle>Configuration</CardTitle></CardHeader>
          <CardContent>
            <Table>
              <TableHeader><TableRow><TableHead>Key</TableHead><TableHead>Value</TableHead></TableRow></TableHeader>
              <TableBody>
                {Object.entries(topic.configs).map(([key, value]) => (
                  <TableRow key={key}>
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
