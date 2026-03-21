import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface SchemaRegistryStepProps {
  schemaRegistry?: { url: string };
  onChange: (sr?: { url: string }) => void;
}

export function SchemaRegistryStep({ schemaRegistry, onChange }: SchemaRegistryStepProps) {
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="sr-url">Schema Registry URL</Label>
        <Input
          id="sr-url"
          value={schemaRegistry?.url ?? ""}
          onChange={(e) => onChange(e.target.value ? { url: e.target.value } : undefined)}
          placeholder="http://schema-registry:8081"
        />
      </div>
    </div>
  );
}
