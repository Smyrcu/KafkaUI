export function getConnectorStateBadgeVariant(state: string) {
  switch (state.toUpperCase()) {
    case "RUNNING": return "success" as const;
    case "PAUSED": return "secondary" as const;
    case "FAILED": return "destructive" as const;
    default: return "outline" as const;
  }
}
