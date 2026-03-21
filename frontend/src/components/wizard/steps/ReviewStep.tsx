import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { CheckCircle2, XCircle, Loader2 } from "lucide-react";
import type { AddClusterRequest, TestConnectionResult } from "@/lib/api";

interface ReviewStepProps {
  data: AddClusterRequest;
  testResult: TestConnectionResult | null;
  testing: boolean;
  onTest: () => void;
}

export function ReviewStep({ data, testResult, testing, onTest }: ReviewStepProps) {
  return (
    <div className="space-y-4">
      <div className="rounded-lg border p-4 space-y-3">
        <div className="flex justify-between">
          <span className="text-sm text-muted-foreground">Name</span>
          <span className="text-sm font-medium">{data.name}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-sm text-muted-foreground">Bootstrap Servers</span>
          <span className="text-sm font-medium">{data.bootstrapServers}</span>
        </div>
        {data.tls?.enabled && (
          <div className="flex justify-between">
            <span className="text-sm text-muted-foreground">TLS</span>
            <Badge variant="secondary">Enabled</Badge>
          </div>
        )}
        {data.sasl && (
          <div className="flex justify-between">
            <span className="text-sm text-muted-foreground">SASL</span>
            <Badge variant="secondary">{data.sasl.mechanism}</Badge>
          </div>
        )}
        {data.schemaRegistry?.url && (
          <div className="flex justify-between">
            <span className="text-sm text-muted-foreground">Schema Registry</span>
            <span className="text-sm">{data.schemaRegistry.url}</span>
          </div>
        )}
        {data.kafkaConnect && data.kafkaConnect.length > 0 && (
          <div className="flex justify-between">
            <span className="text-sm text-muted-foreground">Kafka Connect</span>
            <span className="text-sm">{data.kafkaConnect.length} cluster(s)</span>
          </div>
        )}
        {data.ksql?.url && (
          <div className="flex justify-between">
            <span className="text-sm text-muted-foreground">KSQL</span>
            <span className="text-sm">{data.ksql.url}</span>
          </div>
        )}
      </div>

      <div className="flex items-center gap-3">
        <Button variant="outline" onClick={onTest} disabled={testing}>
          {testing ? (
            <><Loader2 className="h-4 w-4 mr-2 animate-spin" /> Testing...</>
          ) : (
            "Test Connection"
          )}
        </Button>
        {testResult && (
          <div className="flex items-center gap-2 text-sm">
            {testResult.status === "ok" ? (
              <><CheckCircle2 className="h-4 w-4 text-green-500" /> Connection successful</>
            ) : (
              <><XCircle className="h-4 w-4 text-destructive" /> {testResult.error || "Connection failed"}</>
            )}
          </div>
        )}
        {!testResult && !testing && (
          <p className="text-xs text-amber-500">Test connection before saving to verify your configuration</p>
        )}
      </div>
    </div>
  );
}
