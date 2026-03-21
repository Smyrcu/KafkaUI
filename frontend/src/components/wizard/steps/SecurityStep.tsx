import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";

interface SecurityStepProps {
  tls?: { enabled: boolean; caFile?: string };
  onChange: (tls?: { enabled: boolean; caFile?: string }) => void;
}

export function SecurityStep({ tls, onChange }: SecurityStepProps) {
  const enabled = tls?.enabled ?? false;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Checkbox
          id="tls-enabled"
          checked={enabled}
          onCheckedChange={(checked) =>
            onChange(checked === true ? { enabled: true, caFile: tls?.caFile } : undefined)
          }
        />
        <Label htmlFor="tls-enabled">Enable TLS</Label>
      </div>
      {enabled && (
        <div className="space-y-2">
          <Label htmlFor="ca-file">CA File Path</Label>
          <Input
            id="ca-file"
            value={tls?.caFile ?? ""}
            onChange={(e) => onChange({ enabled: true, caFile: e.target.value || undefined })}
            placeholder="/path/to/ca.crt"
          />
        </div>
      )}
    </div>
  );
}
