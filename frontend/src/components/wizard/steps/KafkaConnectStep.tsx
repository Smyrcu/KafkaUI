import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Plus, Trash2 } from "lucide-react";

interface KafkaConnectStepProps {
  kafkaConnect?: { name: string; url: string }[];
  onChange: (kc?: { name: string; url: string }[]) => void;
}

export function KafkaConnectStep({ kafkaConnect, onChange }: KafkaConnectStepProps) {
  const entries = kafkaConnect ?? [];

  const add = () => onChange([...entries, { name: "", url: "" }]);
  const remove = (i: number) => {
    const next = entries.filter((_, idx) => idx !== i);
    onChange(next.length > 0 ? next : undefined);
  };
  const update = (i: number, field: "name" | "url", value: string) => {
    const next = [...entries];
    next[i] = { ...next[i], [field]: value };
    onChange(next);
  };

  return (
    <div className="space-y-4">
      <Label>Kafka Connect Clusters</Label>
      {entries.map((entry, i) => (
        <div key={i} className="flex gap-2 items-end">
          <div className="flex-1 space-y-1">
            <Label className="text-xs">Name</Label>
            <Input value={entry.name} onChange={(e) => update(i, "name", e.target.value)} placeholder="connect-1" />
          </div>
          <div className="flex-1 space-y-1">
            <Label className="text-xs">URL</Label>
            <Input value={entry.url} onChange={(e) => update(i, "url", e.target.value)} placeholder="http://connect:8083" />
          </div>
          <Button variant="ghost" size="icon" onClick={() => remove(i)}>
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
        </div>
      ))}
      <Button variant="outline" size="sm" onClick={add}>
        <Plus className="h-4 w-4 mr-1" /> Add Connect Cluster
      </Button>
    </div>
  );
}
