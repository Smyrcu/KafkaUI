import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api, type KsqlResponse } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";

const quickActions = [
  { label: "SHOW STREAMS", query: "SHOW STREAMS;" },
  { label: "SHOW TABLES", query: "SHOW TABLES;" },
  { label: "SHOW TOPICS", query: "SHOW TOPICS;" },
  { label: "SHOW QUERIES", query: "SHOW QUERIES;" },
];

function formatResultData(data: unknown): string {
  try {
    if (typeof data === "string") {
      const parsed = JSON.parse(data);
      return JSON.stringify(parsed, null, 2);
    }
    return JSON.stringify(data, null, 2);
  } catch {
    return String(data);
  }
}

export function KsqlPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const [query, setQuery] = useState("");
  const [result, setResult] = useState<KsqlResponse | null>(null);

  const executeMutation = useMutation({
    mutationFn: () => api.ksql.execute(clusterName!, { query }),
    onSuccess: (data) => setResult(data),
  });

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "KSQL" },
  ];

  return (
    <div>
      <PageHeader
        title="KSQL"
        description={`Execute KSQL queries on ${clusterName}`}
        breadcrumbs={breadcrumbs}
      />

      <div className="flex flex-wrap gap-2 mb-4">
        {quickActions.map((action) => (
          <Button
            key={action.label}
            variant="outline"
            size="sm"
            onClick={() => setQuery(action.query)}
          >
            {action.label}
          </Button>
        ))}
      </div>

      <div className="space-y-4">
        <Textarea
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Enter KSQL statement... (e.g., SHOW STREAMS;)"
          rows={12}
          className="font-mono"
        />

        <Button
          onClick={() => executeMutation.mutate()}
          disabled={executeMutation.isPending || !query.trim()}
        >
          {executeMutation.isPending ? "Executing..." : "Execute"}
        </Button>
      </div>

      {executeMutation.error && (
        <div className="mt-4">
          <ErrorAlert message={(executeMutation.error as Error).message} />
        </div>
      )}

      {result && (
        <Card className="mt-6 animate-scale-in">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              Result
              {result.type && <Badge variant="secondary">{result.type}</Badge>}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="font-mono text-sm bg-muted rounded-lg p-4 overflow-auto">
              {formatResultData(result.data)}
            </pre>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
