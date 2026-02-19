import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link } from "react-router-dom";
import { api, type CreateTopicRequest } from "@/lib/api";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger, DialogFooter } from "@/components/ui/dialog";
import { Plus, Trash2 } from "lucide-react";

export function TopicsPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [newTopic, setNewTopic] = useState<CreateTopicRequest>({ name: "", partitions: 1, replicas: 1 });

  const { data: topics, isLoading, error } = useQuery({
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

  const filteredTopics = topics?.filter((t) => t.name.toLowerCase().includes(search.toLowerCase()));
  if (isLoading) return <div className="text-muted-foreground">Loading topics...</div>;
  if (error) return <div className="text-destructive">Error: {(error as Error).message}</div>;

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold">Topics</h2>
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
            {createMutation.isError && <p className="text-sm text-destructive mt-2">{(createMutation.error as Error).message}</p>}
          </DialogContent>
        </Dialog>
      </div>
      <div className="mb-4">
        <Input placeholder="Search topics..." value={search} onChange={(e) => setSearch(e.target.value)} className="max-w-sm" />
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Partitions</TableHead>
            <TableHead>Replicas</TableHead>
            <TableHead>Internal</TableHead>
            <TableHead className="w-[80px]">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {filteredTopics?.map((topic) => (
            <TableRow key={topic.name}>
              <TableCell>
                <Link to={`/clusters/${clusterName}/topics/${topic.name}`} className="text-primary hover:underline font-medium">{topic.name}</Link>
              </TableCell>
              <TableCell>{topic.partitions}</TableCell>
              <TableCell>{topic.replicas}</TableCell>
              <TableCell>{topic.internal && <Badge variant="secondary">internal</Badge>}</TableCell>
              <TableCell>
                {!topic.internal && (
                  <Button variant="ghost" size="icon" onClick={() => { if (confirm(`Delete topic "${topic.name}"?`)) deleteMutation.mutate(topic.name); }}>
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
