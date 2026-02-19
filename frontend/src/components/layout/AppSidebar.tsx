import {
  Sidebar, SidebarContent, SidebarGroup, SidebarGroupContent,
  SidebarGroupLabel, SidebarHeader, SidebarMenu, SidebarMenuButton, SidebarMenuItem,
} from "@/components/ui/sidebar";
import { Link, useParams } from "react-router-dom";
import { Database, Server, FileText, Users, Shield, PlugZap, Terminal, BookOpen } from "lucide-react";

const navItems = [
  { label: "Brokers", icon: Server, path: "brokers" },
  { label: "Topics", icon: FileText, path: "topics" },
  { label: "Consumer Groups", icon: Users, path: "consumer-groups" },
  { label: "Schema Registry", icon: BookOpen, path: "schemas" },
  { label: "Kafka Connect", icon: PlugZap, path: "connect" },
  { label: "KSQL", icon: Terminal, path: "ksql" },
  { label: "ACL", icon: Shield, path: "acl" },
];

export function AppSidebar() {
  const { clusterName } = useParams();
  return (
    <Sidebar>
      <SidebarHeader>
        <Link to="/" className="flex items-center gap-2 px-2 py-1">
          <Database className="h-6 w-6" />
          <span className="text-lg font-bold">KafkaUI</span>
        </Link>
      </SidebarHeader>
      <SidebarContent>
        {clusterName && (
          <SidebarGroup>
            <SidebarGroupLabel>{clusterName}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {navItems.map((item) => (
                  <SidebarMenuItem key={item.path}>
                    <SidebarMenuButton asChild>
                      <Link to={`/clusters/${clusterName}/${item.path}`}>
                        <item.icon className="h-4 w-4" />
                        <span>{item.label}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}
      </SidebarContent>
    </Sidebar>
  );
}
