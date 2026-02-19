import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Link } from "react-router-dom";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Database } from "lucide-react";

export function ClustersPage() {
  const { data: clusters, isLoading, error } = useQuery({
    queryKey: ["clusters"],
    queryFn: api.clusters.list,
  });
  if (isLoading) return <div className="text-muted-foreground">Loading clusters...</div>;
  if (error) return <div className="text-destructive">Error: {(error as Error).message}</div>;
  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Clusters</h2>
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {clusters?.map((cluster) => (
          <Link key={cluster.name} to={`/clusters/${cluster.name}/brokers`}>
            <Card className="hover:border-primary transition-colors cursor-pointer">
              <CardHeader className="flex flex-row items-center gap-3">
                <Database className="h-5 w-5 text-muted-foreground" />
                <CardTitle className="text-lg">{cluster.name}</CardTitle>
              </CardHeader>
              <CardContent>
                <Badge variant="secondary">{cluster.bootstrapServers}</Badge>
              </CardContent>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  );
}
