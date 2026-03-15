import { useQuery } from "@tanstack/react-query";
import { api, type PermissionsResponse } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";

export function usePermissions() {
  const { user } = useAuth();
  return useQuery<PermissionsResponse>({
    queryKey: ["permissions"],
    queryFn: () => api.auth.permissions(),
    enabled: user?.authenticated === true,
    staleTime: 30_000,
  });
}

export function useHasAction(action: string): boolean {
  const { data } = usePermissions();
  if (!data) return true; // permissive when loading or auth disabled
  return data.actions.includes(action) || data.actions.includes("*");
}
