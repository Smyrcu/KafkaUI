import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { AppSidebar } from "./AppSidebar";
import { Outlet } from "react-router-dom";
import { Separator } from "@/components/ui/separator";

export function Layout() {
  return (
    <SidebarProvider>
      <AppSidebar />
      <main className="flex-1 flex flex-col min-h-screen">
        <header className="flex h-14 items-center gap-2 border-b px-4">
          <SidebarTrigger />
          <Separator orientation="vertical" className="h-6" />
          <h1 className="text-sm font-medium">Kafka UI</h1>
        </header>
        <div className="flex-1 p-6">
          <Outlet />
        </div>
      </main>
    </SidebarProvider>
  );
}
