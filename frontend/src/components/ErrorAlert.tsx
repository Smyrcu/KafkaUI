import { AlertCircle } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";

function friendlyMessage(msg: string): string {
  if (msg.includes("connection refused")) return "Unable to connect to the Kafka broker. Please verify that Kafka is running and accessible.";
  if (msg.includes("network") || msg.includes("fetch")) return "Network error. Please check your connection.";
  if (msg.includes("timeout")) return "Request timed out. The server might be overloaded.";
  if (msg.includes("not found")) return "The requested resource was not found.";
  return msg;
}

export function ErrorAlert({ message }: { message: string }) {
  return (
    <Card className="border-destructive/50 bg-destructive/5">
      <CardContent className="flex items-center gap-3 pt-6">
        <AlertCircle className="h-5 w-5 text-destructive shrink-0" />
        <div>
          <p className="font-medium text-destructive">Something went wrong</p>
          <p className="text-sm text-muted-foreground mt-1">{friendlyMessage(message)}</p>
        </div>
      </CardContent>
    </Card>
  );
}
