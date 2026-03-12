import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api, type ResetOffsetsRequest } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger, DialogFooter } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";
import { StatCard } from "@/components/StatCard";
import { DetailSkeleton } from "@/components/PageSkeleton";
import { EmptyState } from "@/components/EmptyState";
import { RotateCcw, Users, Activity, Server } from "lucide-react";
import { rowClassName } from "@/lib/utils";
import { getErrorMessage } from "@/lib/error-utils";

export function ConsumerGroupDetailPage() {
  const { clusterName, groupName } = useParams<{ clusterName: string; groupName: string }>();
  const queryClient = useQueryClient();
  const [resetOpen, setResetOpen] = useState(false);
  const [resetTopic, setResetTopic] = useState("");
  const [resetTo, setResetTo] = useState("earliest");

  const { data: group, isLoading, error, refetch } = useQuery({
    queryKey: ["consumer-group", clusterName, groupName],
    queryFn: () => api.consumerGroups.details(clusterName!, groupName!),
    enabled: !!clusterName && !!groupName,
  });

  const resetMutation = useMutation({
    mutationFn: (data: ResetOffsetsRequest) =>
      api.consumerGroups.resetOffsets(clusterName!, groupName!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["consumer-group", clusterName, groupName] });
      setResetOpen(false);
    },
  });

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Consumer Groups", href: `/clusters/${clusterName}/consumer-groups` },
    { label: groupName! },
  ];

  if (isLoading) return <><PageHeader title={groupName!} breadcrumbs={breadcrumbs} /><DetailSkeleton /></>;
  if (error) return <><PageHeader title={groupName!} breadcrumbs={breadcrumbs} /><ErrorAlert error={error} onRetry={() => refetch()} /></>;
  if (!group) return null;

  const availableTopics = group.offsets.map(o => o.topic);
  const totalLag = group.offsets.reduce((sum, o) => sum + o.totalLag, 0);

  return (
    <div className="space-y-6">
      <PageHeader
        title={group.name}
        breadcrumbs={breadcrumbs}
        actions={
          <div className="flex items-center gap-2">
            <Badge variant={group.state === "Stable" ? "success" : group.state === "Empty" ? "secondary" : "destructive"}>
              {group.state}
            </Badge>
            <Dialog open={resetOpen} onOpenChange={setResetOpen}>
              <DialogTrigger asChild>
                <Button variant="outline">
                  <RotateCcw className="h-4 w-4 mr-2" />
                  Reset Offsets
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Reset Consumer Group Offsets</DialogTitle>
                </DialogHeader>
                <div className="grid gap-4 py-4">
                  <div className="grid gap-2">
                    <Label>Topic</Label>
                    <Select value={resetTopic} onValueChange={setResetTopic}>
                      <SelectTrigger>
                        <SelectValue placeholder="Select a topic..." />
                      </SelectTrigger>
                      <SelectContent>
                        {availableTopics.map(t => (
                          <SelectItem key={t} value={t}>{t}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="grid gap-2">
                    <Label>Reset To</Label>
                    <Select value={resetTo} onValueChange={setResetTo}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="earliest">Earliest</SelectItem>
                        <SelectItem value="latest">Latest</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <DialogFooter>
                  <Button
                    onClick={() => resetMutation.mutate({ topic: resetTopic, resetTo })}
                    disabled={!resetTopic || resetMutation.isPending}
                  >
                    {resetMutation.isPending ? "Resetting..." : "Reset"}
                  </Button>
                </DialogFooter>
                {resetMutation.isError && (
                  <p className="text-sm text-destructive mt-2">{getErrorMessage(resetMutation.error)}</p>
                )}
              </DialogContent>
            </Dialog>
          </div>
        }
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <StatCard label="Members" value={group.members.length} icon={Users} accent="primary" />
        <StatCard label="Total Lag" value={totalLag.toLocaleString()} icon={Activity} accent={totalLag > 0 ? "warning" : "success"} />
        <StatCard label="Coordinator" value={`Broker ${group.coordinatorId}`} icon={Server} accent="primary" />
      </div>

      {/* Members */}
      <Card className="animate-scale-in">
        <CardHeader>
          <CardTitle>Members ({group.members.length})</CardTitle>
        </CardHeader>
        <CardContent>
          {group.members.length === 0 ? (
            <EmptyState icon={Users} title="No active members" description="This consumer group has no connected members." />
          ) : (
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>Member ID</TableHead>
                  <TableHead>Client ID</TableHead>
                  <TableHead>Host</TableHead>
                  <TableHead>Topics</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {group.members.map((m, i) => (
                  <TableRow key={m.id} className={rowClassName(i)}>
                    <TableCell className="font-mono text-xs">{m.id}</TableCell>
                    <TableCell>{m.clientId}</TableCell>
                    <TableCell>{m.host}</TableCell>
                    <TableCell>{m.topics?.join(", ") || "-"}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Topic Offsets */}
      {group.offsets.map((topicOffset) => (
        <Card key={topicOffset.topic} className="animate-scale-in">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-base">{topicOffset.topic}</CardTitle>
              <Badge variant={topicOffset.totalLag > 0 ? "warning" : "success"}>
                Total Lag: {topicOffset.totalLag.toLocaleString()}
              </Badge>
            </div>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>Partition</TableHead>
                  <TableHead>Current Offset</TableHead>
                  <TableHead>End Offset</TableHead>
                  <TableHead>Lag</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {topicOffset.partitions.map((p, i) => (
                  <TableRow key={p.partition} className={rowClassName(i)}>
                    <TableCell>
                      <Badge variant="outline">{p.partition}</Badge>
                    </TableCell>
                    <TableCell>{p.currentOffset.toLocaleString()}</TableCell>
                    <TableCell>{p.endOffset.toLocaleString()}</TableCell>
                    <TableCell>
                      <Badge variant={p.lag > 0 ? "warning" : "success"}>
                        {p.lag.toLocaleString()}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
