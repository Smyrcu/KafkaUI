import { Link, useParams, useLocation } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  Database,
  Server,
  FileText,
  Users,
  Shield,
  UserCog,
  PlugZap,
  Terminal,
  BookOpen,
  LayoutDashboard,
  Layers,
  BarChart3,
  Settings,
} from "lucide-react";
import { api } from "@/lib/api";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
} from "@/components/ui/sidebar";

const clusterNavItems = [
  { label: "Brokers", icon: Server, path: "brokers" },
  { label: "Topics", icon: FileText, path: "topics" },
  { label: "Consumer Groups", icon: Users, path: "consumer-groups" },
  { label: "Schema Registry", icon: BookOpen, path: "schemas" },
  { label: "Kafka Connect", icon: PlugZap, path: "connect" },
  { label: "KSQL", icon: Terminal, path: "ksql" },
  { label: "ACL", icon: Shield, path: "acl" },
  { label: "Users", icon: UserCog, path: "users" },
  { label: "Metrics", icon: BarChart3, path: "metrics" },
];

export function AppSidebar() {
  const { clusterName } = useParams();
  const location = useLocation();

  const { data: clusters } = useQuery({
    queryKey: ["clusters"],
    queryFn: api.clusters.list,
  });

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild tooltip="KafkaUI">
              <Link to="/">
                <div className="flex items-center justify-center rounded-md bg-primary p-1.5 text-primary-foreground">
                  <Database className="h-4 w-4" />
                </div>
                <div className="flex flex-col gap-0.5 leading-none">
                  <span className="font-semibold">KafkaUI</span>
                  <span className="text-xs opacity-70">Dashboard</span>
                </div>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              <SidebarMenuItem>
                <SidebarMenuButton
                  asChild
                  isActive={location.pathname === "/"}
                  tooltip="Dashboard"
                >
                  <Link to="/">
                    <LayoutDashboard className="h-4 w-4" />
                    <span>Dashboard</span>
                  </Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
              <SidebarMenuItem>
                <SidebarMenuButton
                  asChild
                  isActive={location.pathname === "/clusters"}
                  tooltip="Clusters"
                >
                  <Link to="/clusters">
                    <Layers className="h-4 w-4" />
                    <span>Clusters</span>
                  </Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
              <SidebarMenuItem>
                <SidebarMenuButton
                  asChild
                  isActive={location.pathname.startsWith("/settings")}
                  tooltip="Settings"
                >
                  <Link to="/settings/clusters">
                    <Settings className="h-4 w-4" />
                    <span>Settings</span>
                  </Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        {clusters && clusters.length > 0 && (
          <>
            <SidebarSeparator />
            <SidebarGroup>
              <SidebarGroupLabel>Clusters</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {clusters.map((cluster) => (
                    <SidebarMenuItem key={cluster.name}>
                      <SidebarMenuButton
                        asChild
                        isActive={clusterName === cluster.name && !clusterNavItems.some(
                          (item) => location.pathname.startsWith(`/clusters/${cluster.name}/${item.path}`)
                        )}
                        tooltip={cluster.name}
                      >
                        <Link to={`/clusters/${cluster.name}/brokers`}>
                          <Database className="h-4 w-4" />
                          <span>{cluster.name}</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  ))}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </>
        )}

        {clusterName && (
          <>
            <SidebarSeparator />
            <SidebarGroup>
              <SidebarGroupLabel>{clusterName}</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {clusterNavItems.map((item) => {
                    const href = `/clusters/${clusterName}/${item.path}`;
                    const isActive = location.pathname.startsWith(href);
                    return (
                      <SidebarMenuItem key={item.path}>
                        <SidebarMenuButton
                          asChild
                          isActive={isActive}
                          tooltip={item.label}
                        >
                          <Link to={href}>
                            <item.icon className="h-4 w-4" />
                            <span>{item.label}</span>
                          </Link>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    );
                  })}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </>
        )}
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="sm" className="text-xs opacity-50 pointer-events-none">
              <span>v0.1.0</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  );
}
