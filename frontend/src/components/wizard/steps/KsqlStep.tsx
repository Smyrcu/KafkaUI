import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface KsqlStepProps {
  ksql?: { url: string };
  onChange: (ksql?: { url: string }) => void;
}

function validateURL(value: string): string | null {
  if (!value) return null;
  try {
    const u = new URL(value);
    if (!["http:", "https:"].includes(u.protocol)) return "URL must start with http:// or https://";
    return null;
  } catch {
    return "Invalid URL format (e.g. http://ksql:8088)";
  }
}

export function KsqlStep({ ksql, onChange }: KsqlStepProps) {
  const urlError = validateURL(ksql?.url ?? "");
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="ksql-url">KSQL Server URL</Label>
        <Input
          id="ksql-url"
          value={ksql?.url ?? ""}
          onChange={(e) => onChange(e.target.value ? { url: e.target.value } : undefined)}
          placeholder="http://ksql:8088"
          className={urlError ? "border-destructive" : ""}
        />
        {urlError && <p className="text-xs text-destructive">{urlError}</p>}
      </div>
    </div>
  );
}
