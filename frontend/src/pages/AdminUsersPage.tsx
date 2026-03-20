import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type AdminUser, type SetRolesRequest } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PageHeader } from "@/components/PageHeader";
import { ErrorAlert } from "@/components/ErrorAlert";
import { EmptyState } from "@/components/EmptyState";
import { TableSkeleton } from "@/components/PageSkeleton";
import { getErrorMessage } from "@/lib/error-utils";
import { Users, Trash2, Shield } from "lucide-react";
import { useHasAction } from "@/hooks/usePermissions";

const COMMON_ROLES = ["admin", "operator", "viewer"] as const;

function formatLastLogin(ts: string): string {
  if (!ts) return "—";
  try {
    return new Date(ts).toLocaleString();
  } catch {
    return ts;
  }
}

interface EditRolesDialogProps {
  user: AdminUser;
  onClose: () => void;
}

function EditRolesDialog({ user, onClose }: EditRolesDialogProps) {
  const queryClient = useQueryClient();
  const [checked, setChecked] = useState<Set<string>>(new Set(user.roles));
  const [customInput, setCustomInput] = useState("");

  const setRolesMutation = useMutation({
    mutationFn: (data: SetRolesRequest) => api.admin.setUserRoles(user.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin-users"] });
      onClose();
    },
  });

  const toggle = (role: string) => {
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(role)) {
        next.delete(role);
      } else {
        next.add(role);
      }
      return next;
    });
  };

  const addCustomRole = () => {
    const role = customInput.trim();
    if (role && !checked.has(role)) {
      setChecked((prev) => new Set([...prev, role]));
    }
    setCustomInput("");
  };

  const removeRole = (role: string) => {
    setChecked((prev) => {
      const next = new Set(prev);
      next.delete(role);
      return next;
    });
  };

  const customRoles = [...checked].filter((r) => !(COMMON_ROLES as readonly string[]).includes(r));

  const handleSave = () => {
    setRolesMutation.mutate({ roles: [...checked] });
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit Roles — {user.name || user.email}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <p className="text-sm font-medium">Common roles</p>
            {COMMON_ROLES.map((role) => (
              <div key={role} className="flex items-center gap-2">
                <Checkbox
                  id={`role-${role}`}
                  checked={checked.has(role)}
                  onCheckedChange={() => toggle(role)}
                />
                <Label htmlFor={`role-${role}`} className="font-normal capitalize">{role}</Label>
              </div>
            ))}
          </div>

          <div className="space-y-2">
            <p className="text-sm font-medium">Custom roles</p>
            <div className="flex gap-2">
              <Input
                value={customInput}
                onChange={(e) => setCustomInput(e.target.value)}
                placeholder="custom-role"
                onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addCustomRole(); } }}
              />
              <Button variant="outline" onClick={addCustomRole} disabled={!customInput.trim()}>
                Add
              </Button>
            </div>
            {customRoles.length > 0 && (
              <div className="flex flex-wrap gap-1 pt-1">
                {customRoles.map((role) => (
                  <Badge key={role} variant="secondary" className="gap-1">
                    {role}
                    <button
                      onClick={() => removeRole(role)}
                      className="ml-1 hover:text-destructive"
                      aria-label={`Remove ${role}`}
                    >
                      ×
                    </button>
                  </Badge>
                ))}
              </div>
            )}
          </div>

          {setRolesMutation.isError && (
            <p className="text-sm text-destructive">{getErrorMessage(setRolesMutation.error)}</p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSave} disabled={setRolesMutation.isPending}>
            {setRolesMutation.isPending ? "Saving..." : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function AdminUsersPage() {
  const canManageUsers = useHasAction("manage_users");
  const queryClient = useQueryClient();
  const [editUser, setEditUser] = useState<AdminUser | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<AdminUser | null>(null);

  const { data: users, isLoading, error, refetch } = useQuery({
    queryKey: ["admin-users"],
    queryFn: api.admin.listUsers,
    enabled: canManageUsers,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.admin.deleteUser(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin-users"] });
      setDeleteTarget(null);
    },
  });

  const breadcrumbs = [
    { label: "Dashboard", href: "/" },
    { label: "Settings" },
  ];

  if (!canManageUsers) {
    return (
      <div>
        <PageHeader title="Admin Users" breadcrumbs={breadcrumbs} />
        <p className="p-4 text-sm text-muted-foreground">Access denied.</p>
      </div>
    );
  }

  if (isLoading) return <><PageHeader title="Admin Users" breadcrumbs={breadcrumbs} /><TableSkeleton cols={5} /></>;
  if (error) {
    return (
      <div>
        <PageHeader title="Admin Users" breadcrumbs={breadcrumbs} />
        <ErrorAlert error={error} onRetry={() => refetch()} />
      </div>
    );
  }

  const userList = users ?? [];

  return (
    <div>
      <PageHeader
        title="Admin Users"
        description="Manage user roles for authenticated users"
        breadcrumbs={breadcrumbs}
      />

      {userList.length === 0 ? (
        <EmptyState
          icon={Users}
          title="No users found"
          description="Users appear here after they first log in via an authentication provider."
        />
      ) : (
        <div className="rounded-lg border bg-card animate-scale-in">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead>Roles</TableHead>
                <TableHead>Last Login</TableHead>
                <TableHead className="w-32"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {userList.map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="font-medium">{user.name || "—"}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">{user.email || "—"}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{user.providerName}</Badge>
                  </TableCell>
                  <TableCell>
                    {user.roles.length === 0 ? (
                      <span className="text-xs text-muted-foreground">no roles</span>
                    ) : (
                      <div className="flex flex-wrap gap-1">
                        {user.roles.map((role) => (
                          <Badge key={role} variant="secondary">{role}</Badge>
                        ))}
                      </div>
                    )}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {formatLastLogin(user.lastLogin)}
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setEditUser(user)}
                        title="Edit Roles"
                        aria-label="Edit roles"
                      >
                        <Shield className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setDeleteTarget(user)}
                        title="Delete User"
                        aria-label="Delete user"
                        className="text-destructive hover:text-destructive"
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {editUser && (
        <EditRolesDialog user={editUser} onClose={() => setEditUser(null)} />
      )}

      {/* Delete Confirmation Dialog */}
      <Dialog open={!!deleteTarget} onOpenChange={() => setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete User</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Are you sure you want to delete user{" "}
            <span className="font-medium text-foreground">
              {deleteTarget?.name || deleteTarget?.email}
            </span>
            ? This action cannot be undone.
          </p>
          {deleteMutation.isError && (
            <p className="text-sm text-destructive">{getErrorMessage(deleteMutation.error)}</p>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>Cancel</Button>
            <Button
              variant="destructive"
              disabled={deleteMutation.isPending}
              onClick={() => { if (deleteTarget) deleteMutation.mutate(deleteTarget.id); }}
            >
              {deleteMutation.isPending ? "Deleting..." : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
