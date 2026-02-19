import { Link, useParams, useLocation } from "react-router-dom";
import { Database, Server, FileText, Users, Shield, PlugZap, Terminal, BookOpen } from "lucide-react";
import { cn } from "@/lib/utils";

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
  const location = useLocation();

  return (
    <aside className="w-64 shrink-0 border-r bg-muted/30 flex flex-col">
      <div className="h-14 shrink-0 flex items-center gap-2 px-4 border-b">
        <Link to="/" className="flex items-center gap-2">
          <Database className="h-5 w-5" />
          <span className="text-lg font-bold tracking-tight">KafkaUI</span>
        </Link>
      </div>
      <nav className="flex-1 overflow-auto py-2">
        {clusterName && (
          <div className="px-3 py-2">
            <p className="text-[0.65rem] font-semibold uppercase tracking-wider text-muted-foreground px-2 mb-2">
              {clusterName}
            </p>
            <ul className="space-y-1">
              {navItems.map((item) => {
                const href = `/clusters/${clusterName}/${item.path}`;
                const isActive = location.pathname.startsWith(href);
                return (
                  <li key={item.path}>
                    <Link
                      to={href}
                      className={cn(
                        "flex items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors",
                        isActive
                          ? "bg-accent text-accent-foreground font-medium"
                          : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                      )}
                    >
                      <item.icon className="h-4 w-4 shrink-0" />
                      <span>{item.label}</span>
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        )}
      </nav>
    </aside>
  );
}
