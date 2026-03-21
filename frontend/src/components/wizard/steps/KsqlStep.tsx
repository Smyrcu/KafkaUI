import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface KsqlStepProps {
  ksql?: { url: string };
  onChange: (ksql?: { url: string }) => void;
}

export function KsqlStep({ ksql, onChange }: KsqlStepProps) {
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="ksql-url">KSQL Server URL</Label>
        <Input
          id="ksql-url"
          value={ksql?.url ?? ""}
          onChange={(e) => onChange(e.target.value ? { url: e.target.value } : undefined)}
          placeholder="http://ksql:8088"
        />
      </div>
    </div>
  );
}
