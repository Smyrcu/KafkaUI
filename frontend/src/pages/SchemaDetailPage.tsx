import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api } from "@/lib/api";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ErrorAlert } from "@/components/ErrorAlert";
import { PageHeader } from "@/components/PageHeader";
import { StatCard } from "@/components/StatCard";
import { DetailSkeleton } from "@/components/PageSkeleton";
import { Layers, Hash, FileText } from "lucide-react";

function formatSchema(raw: string): string {
  try {
    const parsed = JSON.parse(raw);
    return JSON.stringify(parsed, null, 2);
  } catch {
    return raw;
  }
}

export function SchemaDetailPage() {
  const { clusterName, subject } = useParams<{ clusterName: string; subject: string }>();
  const [selectedVersion, setSelectedVersion] = useState<number | null>(null);

  const { data: schema, isLoading, error, refetch } = useQuery({
    queryKey: ["schema-detail", clusterName, subject],
    queryFn: () => api.schemas.details(clusterName!, subject!),
    enabled: !!clusterName && !!subject,
  });

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: clusterName!, href: `/clusters/${clusterName}/brokers` },
    { label: "Schema Registry", href: `/clusters/${clusterName}/schemas` },
    { label: subject! },
  ];

  if (isLoading) return <DetailSkeleton />;
  if (error) return <ErrorAlert message={(error as Error).message} onRetry={() => refetch()} />;
  if (!schema) return null;

  const sortedVersions = [...schema.versions].sort((a, b) => b.version - a.version);
  const latestVersion = sortedVersions.length > 0 ? sortedVersions[0] : null;
  const activeVersion = selectedVersion ?? latestVersion?.version ?? null;
  const activeSchema = schema.versions.find((v) => v.version === activeVersion);

  return (
    <div className="space-y-6">
      <PageHeader
        title={schema.subject}
        breadcrumbs={breadcrumbs}
        actions={<Badge variant="secondary">{schema.compatibility}</Badge>}
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <StatCard label="Total Versions" value={schema.versions.length} icon={Layers} accent="primary" />
        <StatCard label="Latest Version" value={latestVersion?.version ?? "-"} icon={Hash} accent="success" />
        <StatCard label="Schema Type" value={latestVersion?.schemaType ?? "-"} icon={FileText} accent="warning" />
      </div>

      {/* Versions Table */}
      <Card className="animate-scale-in">
        <CardHeader>
          <CardTitle>Versions</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>Version</TableHead>
                <TableHead>Schema ID</TableHead>
                <TableHead>Schema Type</TableHead>
                <TableHead></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedVersions.map((v, i) => (
                <TableRow key={v.version} className={i % 2 === 1 ? "bg-muted/30" : ""}>
                  <TableCell>
                    <Badge variant={v.version === activeVersion ? "default" : "outline"}>{v.version}</Badge>
                  </TableCell>
                  <TableCell>{v.id}</TableCell>
                  <TableCell>{v.schemaType}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant={v.version === activeVersion ? "default" : "ghost"}
                      size="sm"
                      onClick={() => setSelectedVersion(v.version)}
                    >
                      {v.version === activeVersion ? "Viewing" : "View"}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Schema Content Display */}
      <Card className="animate-scale-in">
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Schema Content</CardTitle>
            <div className="flex items-center gap-2">
              {sortedVersions.map((v) => (
                <Button
                  key={v.version}
                  variant={v.version === activeVersion ? "default" : "outline"}
                  size="sm"
                  onClick={() => setSelectedVersion(v.version)}
                >
                  v{v.version}
                </Button>
              ))}
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {activeSchema ? (
            <pre className="font-mono text-sm bg-muted rounded-lg p-4 overflow-auto">
              {formatSchema(activeSchema.schema)}
            </pre>
          ) : (
            <p className="text-sm text-muted-foreground py-4 text-center">No schema version selected</p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
