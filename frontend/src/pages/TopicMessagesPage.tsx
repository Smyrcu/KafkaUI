import { useState, Fragment } from "react";
import { useParams } from "react-router-dom";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useHasAction } from "@/hooks/usePermissions";
import { api, type BrowseParams, type ProduceRequest, type MessageRecord } from "@/lib/api";
import { useWebSocket } from "@/hooks/useWebSocket";
import { TopicTabs } from "@/components/TopicTabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger, DialogFooter } from "@/components/ui/dialog";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";
import { Play, Square, Send, Plus, Trash2, ChevronDown, ChevronRight, MessageSquare, Filter, Download } from "lucide-react";
import { rowClassName } from "@/lib/utils";
import { getErrorMessage } from "@/lib/error-utils";

type OffsetMode = "latest" | "earliest" | "timestamp" | "custom";

export function TopicMessagesPage() {
  const { clusterName, topicName } = useParams<{ clusterName: string; topicName: string }>();
  const canProduceMessages = useHasAction("produce_messages");

  // Browse state
  const [partition, setPartition] = useState<string>("");
  const [offsetMode, setOffsetMode] = useState<OffsetMode>("earliest");
  const [customOffset, setCustomOffset] = useState("");
  const [timestamp, setTimestamp] = useState("");
  const [limit, setLimit] = useState(100);
  const [browseParams, setBrowseParams] = useState<BrowseParams | null>(null);
  const [fetchNonce, setFetchNonce] = useState(0);
  const [expandedRow, setExpandedRow] = useState<string | null>(null);
  const [celFilter, setCelFilter] = useState("");

  // Live tail
  const wsUrl = clusterName && topicName
    ? `/ws/clusters/${clusterName}/topics/${topicName}/live`
    : null;
  const { messages: liveMessages, state: wsState, connect, disconnect, clear } = useWebSocket(wsUrl);
  const isLiveTail = wsState === "connected" || wsState === "connecting";

  // Produce state
  const [produceOpen, setProduceOpen] = useState(false);
  const [produceData, setProduceData] = useState<ProduceRequest>({ key: "", value: "" });
  const [produceHeaders, setProduceHeaders] = useState<{ key: string; value: string }[]>([]);

  // Fetch messages
  const { data: fetchedMessages, isLoading, error } = useQuery({
    queryKey: ["messages", clusterName, topicName, browseParams, fetchNonce],
    queryFn: () => api.messages.browse(clusterName!, topicName!, browseParams!),
    enabled: !!clusterName && !!topicName && !!browseParams && !isLiveTail,
  });

  const produceMutation = useMutation({
    mutationFn: (data: ProduceRequest) => api.messages.produce(clusterName!, topicName!, data),
    onSuccess: () => {
      setProduceOpen(false);
      setProduceData({ key: "", value: "" });
      setProduceHeaders([]);
    },
  });

  const handleFetch = () => {
    const params: BrowseParams = { limit };
    if (partition !== "") params.partition = parseInt(partition);
    switch (offsetMode) {
      case "latest": params.offset = "latest"; break;
      case "earliest": params.offset = "earliest"; break;
      case "custom": params.offset = customOffset; break;
      case "timestamp": params.timestamp = new Date(timestamp).toISOString(); break;
    }
    if (celFilter) params.filter = celFilter;
    setBrowseParams({ ...params });
    setFetchNonce((n) => n + 1);
  };

  const handleProduce = () => {
    const headers: Record<string, string> = {};
    produceHeaders.forEach((h) => { if (h.key) headers[h.key] = h.value; });
    produceMutation.mutate({
      ...produceData,
      headers: Object.keys(headers).length > 0 ? headers : undefined,
    });
  };

  const toggleLiveTail = () => {
    if (isLiveTail) {
      disconnect();
    } else {
      setBrowseParams(null);
      connect(celFilter || undefined);
    }
  };

  const messages = isLiveTail ? liveMessages : (fetchedMessages || []);

  const handleDownload = () => {
    if (messages.length === 0) return;
    const partitionLabel = partition || "all";
    const ts = new Date().toISOString().replace(/[:.]/g, "-");
    const filename = `${topicName}_${partitionLabel}_${ts}.json`;

    const blob = new Blob([JSON.stringify(messages, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 100);
  };
  const rowKey = (m: MessageRecord) => `${m.partition}-${m.offset}`;

  const tryFormatJson = (s: string) => {
    try { return JSON.stringify(JSON.parse(s), null, 2); } catch { return s; }
  };

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Topics", href: `/clusters/${clusterName}/topics` },
    { label: topicName!, href: `/clusters/${clusterName}/topics/${topicName}` },
    { label: "Messages" },
  ];

  return (
    <div>
      <PageHeader title={topicName!} breadcrumbs={breadcrumbs} />
      <TopicTabs />

      {/* Toolbar */}
      <div className="flex flex-wrap items-end gap-3 mb-4">
        <div className="grid gap-1">
          <Label className="text-xs">Partition</Label>
          <Input className="w-24" placeholder="All" value={partition} onChange={(e) => setPartition(e.target.value)} disabled={isLiveTail} />
        </div>
        <div className="grid gap-1">
          <Label className="text-xs">Offset</Label>
          <Select value={offsetMode} onValueChange={(v) => setOffsetMode(v as OffsetMode)} disabled={isLiveTail}>
            <SelectTrigger className="w-[130px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="earliest">Earliest</SelectItem>
              <SelectItem value="latest">Latest</SelectItem>
              <SelectItem value="timestamp">Timestamp</SelectItem>
              <SelectItem value="custom">Custom</SelectItem>
            </SelectContent>
          </Select>
        </div>
        {offsetMode === "custom" && (
          <div className="grid gap-1">
            <Label className="text-xs">Offset value</Label>
            <Input className="w-28" value={customOffset} onChange={(e) => setCustomOffset(e.target.value)} disabled={isLiveTail} />
          </div>
        )}
        {offsetMode === "timestamp" && (
          <div className="grid gap-1">
            <Label className="text-xs">Timestamp</Label>
            <Input type="datetime-local" className="w-52" value={timestamp} onChange={(e) => setTimestamp(e.target.value)} disabled={isLiveTail} />
          </div>
        )}
        <div className="grid gap-1">
          <Label className="text-xs">Limit</Label>
          <Input className="w-20" type="number" min={1} max={500} value={limit} onChange={(e) => setLimit(parseInt(e.target.value) || 100)} disabled={isLiveTail} />
        </div>
        <Button onClick={handleFetch} disabled={isLiveTail || isLoading}>
          {isLoading ? "Loading..." : "Fetch"}
        </Button>
        <Button variant={isLiveTail ? "destructive" : "outline"} onClick={toggleLiveTail}>
          {isLiveTail ? <><Square className="h-4 w-4 mr-2" />Stop</> : <><Play className="h-4 w-4 mr-2" />Live Tail</>}
        </Button>
        {isLiveTail && (
          <Button variant="ghost" size="sm" onClick={clear}>Clear ({liveMessages.length})</Button>
        )}
        {messages.length > 0 && (
          <Button variant="outline" onClick={handleDownload}>
            <Download className="h-4 w-4 mr-2" />Download JSON
          </Button>
        )}
        {canProduceMessages && (
        <div className="ml-auto">
          <Dialog open={produceOpen} onOpenChange={setProduceOpen}>
            <DialogTrigger asChild>
              <Button variant="outline"><Send className="h-4 w-4 mr-2" />Produce</Button>
            </DialogTrigger>
            <DialogContent className="max-w-lg">
              <DialogHeader><DialogTitle>Produce Message</DialogTitle></DialogHeader>
              <div className="grid gap-4 py-2">
                <div className="grid gap-2">
                  <Label>Key</Label>
                  <Input value={produceData.key} onChange={(e) => setProduceData({ ...produceData, key: e.target.value })} placeholder="Message key (optional)" />
                </div>
                <div className="grid gap-2">
                  <Label>Value</Label>
                  <Textarea
                    className="min-h-[120px] font-mono"
                    value={produceData.value}
                    onChange={(e) => setProduceData({ ...produceData, value: e.target.value })}
                    placeholder='{"event": "test"}'
                  />
                </div>
                <div className="grid gap-2">
                  <div className="flex items-center justify-between">
                    <Label>Headers</Label>
                    <Button variant="ghost" size="sm" onClick={() => setProduceHeaders([...produceHeaders, { key: "", value: "" }])}>
                      <Plus className="h-3 w-3 mr-1" />Add
                    </Button>
                  </div>
                  {produceHeaders.map((h, i) => (
                    <div key={i} className="flex gap-2">
                      <Input placeholder="Key" value={h.key} onChange={(e) => { const nh = [...produceHeaders]; nh[i] = { ...nh[i], key: e.target.value }; setProduceHeaders(nh); }} />
                      <Input placeholder="Value" value={h.value} onChange={(e) => { const nh = [...produceHeaders]; nh[i] = { ...nh[i], value: e.target.value }; setProduceHeaders(nh); }} />
                      <Button variant="ghost" size="icon" onClick={() => setProduceHeaders(produceHeaders.filter((_, j) => j !== i))}>
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
              <DialogFooter>
                <Button onClick={handleProduce} disabled={produceMutation.isPending}>
                  {produceMutation.isPending ? "Sending..." : "Send"}
                </Button>
              </DialogFooter>
              {produceMutation.isError && <p className="text-sm text-destructive">{getErrorMessage(produceMutation.error)}</p>}
            </DialogContent>
          </Dialog>
        </div>
        )}
      </div>

      {/* CEL Filter */}
      <div className="flex flex-col gap-1.5 mb-4">
        <div className="flex items-center gap-2">
          <Label className="text-xs flex items-center gap-1 shrink-0"><Filter className="h-3 w-3" />CEL Filter</Label>
          <Input
            className="font-mono text-sm"
            placeholder='value.status == "ERROR" && key.contains("order")'
            value={celFilter}
            onChange={(e) => setCelFilter(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter" && !isLiveTail) handleFetch(); }}
          />
        </div>
        <p className="text-xs text-muted-foreground ml-[4.5rem]">
          Fields: <code className="bg-muted px-1 rounded">key</code> <code className="bg-muted px-1 rounded">value</code> <code className="bg-muted px-1 rounded">headers</code> <code className="bg-muted px-1 rounded">partition</code> <code className="bg-muted px-1 rounded">offset</code> <code className="bg-muted px-1 rounded">timestamp</code>
          {" · "}JSON values: <code className="bg-muted px-1 rounded">value.field == "x"</code>
          {" · "}String: <code className="bg-muted px-1 rounded">key.contains("order")</code>
          {" · "}Combine: <code className="bg-muted px-1 rounded">value.amount &gt; 100 && headers.source == "payments"</code>
        </p>
      </div>

      {/* Connection indicator */}
      {isLiveTail && (
        <div className="flex items-center gap-2 mb-3">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
          </span>
          <span className="text-xs text-muted-foreground">Live tail active — {liveMessages.length} messages</span>
        </div>
      )}

      {error && <ErrorAlert error={error} />}

      {/* Message table */}
      {isLoading ? (
        <TableSkeleton rows={5} cols={6} />
      ) : messages.length > 0 ? (
        <div className="rounded-lg border bg-card animate-scale-in">
          <div className="px-4 py-3 border-b">
            <p className="text-sm text-muted-foreground">{messages.length} messages</p>
          </div>
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="w-8"></TableHead>
                <TableHead>Partition</TableHead>
                <TableHead>Offset</TableHead>
                <TableHead>Timestamp</TableHead>
                <TableHead>Key</TableHead>
                <TableHead>Value</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {messages.map((m, i) => {
                const key = rowKey(m);
                const isExpanded = expandedRow === key;
                return (
                  <Fragment key={key}>
                    <TableRow className={`cursor-pointer hover:bg-muted/50 ${rowClassName(i)}`} onClick={() => setExpandedRow(isExpanded ? null : key)}>
                      <TableCell>{isExpanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}</TableCell>
                      <TableCell><Badge variant="outline">{m.partition}</Badge></TableCell>
                      <TableCell className="font-mono text-xs">{m.offset}</TableCell>
                      <TableCell className="text-xs">{new Date(m.timestamp).toLocaleString()}</TableCell>
                      <TableCell className="font-mono text-xs max-w-[150px] truncate">{m.key || <span className="text-muted-foreground">null</span>}</TableCell>
                      <TableCell className="font-mono text-xs max-w-[300px] truncate">{m.value}</TableCell>
                    </TableRow>
                    {isExpanded && (
                      <TableRow>
                        <TableCell colSpan={6} className="bg-muted/30 p-4">
                          <div className="grid gap-3">
                            <div>
                              <p className="text-xs font-semibold mb-1">Key</p>
                              <pre className="text-xs bg-background rounded p-2 border overflow-auto max-h-32">{m.key ? tryFormatJson(m.key) : "null"}</pre>
                            </div>
                            <div>
                              <p className="text-xs font-semibold mb-1">Value</p>
                              <pre className="text-xs bg-background rounded p-2 border overflow-auto max-h-64">{tryFormatJson(m.value)}</pre>
                            </div>
                            {m.headers && Object.keys(m.headers).length > 0 && (
                              <div>
                                <p className="text-xs font-semibold mb-1">Headers</p>
                                <pre className="text-xs bg-background rounded p-2 border overflow-auto">{JSON.stringify(m.headers, null, 2)}</pre>
                              </div>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    )}
                  </Fragment>
                );
              })}
            </TableBody>
          </Table>
        </div>
      ) : (
        !isLiveTail && (
          <EmptyState
            icon={MessageSquare}
            title="No messages"
            description='Click "Fetch" to browse messages or "Live Tail" to stream new messages.'
          />
        )
      )}
    </div>
  );
}
