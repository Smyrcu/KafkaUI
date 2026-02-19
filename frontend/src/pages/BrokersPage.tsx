import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api } from "@/lib/api";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { ErrorAlert } from "@/components/ErrorAlert";

export function BrokersPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const { data: brokers, isLoading, error } = useQuery({
    queryKey: ["brokers", clusterName],
    queryFn: () => api.brokers.list(clusterName!),
    enabled: !!clusterName,
  });
  if (isLoading) return <div className="text-muted-foreground">Loading brokers...</div>;
  if (error) return <ErrorAlert message={(error as Error).message} />;
  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Brokers</h2>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>ID</TableHead>
            <TableHead>Host</TableHead>
            <TableHead>Port</TableHead>
            <TableHead>Rack</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {brokers?.map((broker) => (
            <TableRow key={broker.id}>
              <TableCell><Badge variant="outline">{broker.id}</Badge></TableCell>
              <TableCell>{broker.host}</TableCell>
              <TableCell>{broker.port}</TableCell>
              <TableCell>{broker.rack || "\u2014"}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
