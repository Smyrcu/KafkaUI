import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface ConnectionStepProps {
  name: string;
  bootstrapServers: string;
  onChange: (data: { name: string; bootstrapServers: string }) => void;
}

export function ConnectionStep({ name, bootstrapServers, onChange }: ConnectionStepProps) {
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="cluster-name">Cluster Name *</Label>
        <Input
          id="cluster-name"
          value={name}
          onChange={(e) => onChange({ name: e.target.value, bootstrapServers })}
          placeholder="my-cluster"
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor="bootstrap-servers">Bootstrap Servers *</Label>
        <Input
          id="bootstrap-servers"
          value={bootstrapServers}
          onChange={(e) => onChange({ name, bootstrapServers: e.target.value })}
          placeholder="localhost:9092"
        />
      </div>
    </div>
  );
}
