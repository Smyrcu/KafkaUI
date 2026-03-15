import { useQuery } from "@tanstack/react-query";
import { api, type PermissionsResponse } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";

export function usePermissions() {
  const { user } = useAuth();
  return useQuery<PermissionsResponse>({
    queryKey: ["permissions", user?.email],
    queryFn: () => api.auth.permissions(),
    enabled: user?.authenticated === true,
    staleTime: 30_000,
  });
}

export function useHasAction(action: string): boolean {
  const { status } = useAuth();
  const { data, isLoading } = usePermissions();

  // If auth is not enabled, all actions are allowed
  if (!status?.enabled) return true;

  // Deny during loading to prevent flash of authorized content
  if (isLoading || !data) return false;

  return data.actions.includes(action) || data.actions.includes("*");
}
