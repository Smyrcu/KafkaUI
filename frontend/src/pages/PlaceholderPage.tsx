import { Construction } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";

export function PlaceholderPage({ title }: { title: string }) {
  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">{title}</h2>
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-12 text-muted-foreground">
          <Construction className="h-10 w-10 mb-3" />
          <p className="text-lg font-medium">Coming soon</p>
          <p className="text-sm">This feature is under development.</p>
        </CardContent>
      </Card>
    </div>
  );
}
