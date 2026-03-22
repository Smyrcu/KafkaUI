import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface SchemaRegistryStepProps {
  schemaRegistry?: { url: string };
  onChange: (sr?: { url: string }) => void;
}

function validateURL(value: string): string | null {
  if (!value) return null;
  try {
    const u = new URL(value);
    if (!["http:", "https:"].includes(u.protocol)) return "URL must start with http:// or https://";
    return null;
  } catch {
    return "Invalid URL format (e.g. http://schema-registry:8081)";
  }
}

export function SchemaRegistryStep({ schemaRegistry, onChange }: SchemaRegistryStepProps) {
  const urlError = validateURL(schemaRegistry?.url ?? "");
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="sr-url">Schema Registry URL</Label>
        <Input
          id="sr-url"
          value={schemaRegistry?.url ?? ""}
          onChange={(e) => onChange(e.target.value ? { url: e.target.value } : undefined)}
          placeholder="http://schema-registry:8081"
          className={urlError ? "border-destructive" : ""}
        />
        {urlError && <p className="text-xs text-destructive">{urlError}</p>}
      </div>
    </div>
  );
}
