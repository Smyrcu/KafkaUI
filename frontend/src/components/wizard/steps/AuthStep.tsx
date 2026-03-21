import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

interface AuthStepProps {
  sasl?: { mechanism: string; username: string; password: string };
  onChange: (sasl?: { mechanism: string; username: string; password: string }) => void;
}

const mechanisms = ["PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"];

export function AuthStep({ sasl, onChange }: AuthStepProps) {
  const mechanism = sasl?.mechanism ?? "";

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>SASL Mechanism</Label>
        <Select
          value={mechanism || "none"}
          onValueChange={(v) => {
            if (v === "none") {
              onChange(undefined);
            } else {
              onChange({ mechanism: v, username: sasl?.username ?? "", password: sasl?.password ?? "" });
            }
          }}
        >
          <SelectTrigger>
            <SelectValue placeholder="None" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">None</SelectItem>
            {mechanisms.map((m) => (
              <SelectItem key={m} value={m}>{m}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {mechanism && (
        <>
          <div className="space-y-2">
            <Label htmlFor="sasl-username">Username</Label>
            <Input
              id="sasl-username"
              value={sasl?.username ?? ""}
              onChange={(e) => onChange({ mechanism, username: e.target.value, password: sasl?.password ?? "" })}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="sasl-password">Password</Label>
            <Input
              id="sasl-password"
              type="password"
              value={sasl?.password ?? ""}
              onChange={(e) => onChange({ mechanism, username: sasl?.username ?? "", password: e.target.value })}
            />
          </div>
        </>
      )}
    </div>
  );
}
