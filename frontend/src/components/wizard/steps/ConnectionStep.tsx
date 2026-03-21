import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface ConnectionStepProps {
  name: string;
  bootstrapServers: string;
  onChange: (data: { name: string; bootstrapServers: string }) => void;
}

const BOOTSTRAP_RE = /^[\w.-]+:\d{1,5}(,\s*[\w.-]+:\d{1,5})*$/;

function validateBootstrap(value: string): string | null {
  if (!value.trim()) return null;
  if (!BOOTSTRAP_RE.test(value.trim())) return "Format: host:port (e.g. localhost:9092, broker1:9092,broker2:9092)";
  const ports = value.split(",").map(s => parseInt(s.trim().split(":")[1]));
  if (ports.some(p => p < 1 || p > 65535)) return "Port must be between 1 and 65535";
  return null;
}

function validateName(value: string): string | null {
  if (!value.trim()) return null;
  if (!/^[a-zA-Z0-9_-]+$/.test(value.trim())) return "Only letters, numbers, hyphens, and underscores";
  return null;
}

export function ConnectionStep({ name, bootstrapServers, onChange }: ConnectionStepProps) {
  const nameError = validateName(name);
  const bootstrapError = validateBootstrap(bootstrapServers);

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="cluster-name">Cluster Name *</Label>
        <Input
          id="cluster-name"
          value={name}
          onChange={(e) => onChange({ name: e.target.value, bootstrapServers })}
          placeholder="my-cluster"
          className={nameError ? "border-destructive" : ""}
        />
        {nameError && <p className="text-xs text-destructive">{nameError}</p>}
      </div>
      <div className="space-y-2">
        <Label htmlFor="bootstrap-servers">Bootstrap Servers *</Label>
        <Input
          id="bootstrap-servers"
          value={bootstrapServers}
          onChange={(e) => onChange({ name, bootstrapServers: e.target.value })}
          placeholder="localhost:9092"
          className={bootstrapError ? "border-destructive" : ""}
        />
        {bootstrapError && <p className="text-xs text-destructive">{bootstrapError}</p>}
      </div>
    </div>
  );
}

export { validateBootstrap, validateName };
