import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@/components/ThemeProvider";
import { Layout } from "@/components/layout/Layout";
import { DashboardPage } from "@/pages/DashboardPage";
import { ClustersPage } from "@/pages/ClustersPage";
import { BrokersPage } from "@/pages/BrokersPage";
import { TopicsPage } from "@/pages/TopicsPage";
import { TopicDetailPage } from "@/pages/TopicDetailPage";
import { TopicMessagesPage } from "@/pages/TopicMessagesPage";
import { ConsumerGroupsPage } from "@/pages/ConsumerGroupsPage";
import { ConsumerGroupDetailPage } from "@/pages/ConsumerGroupDetailPage";
import { SchemaRegistryPage } from "@/pages/SchemaRegistryPage";
import { SchemaDetailPage } from "@/pages/SchemaDetailPage";
import { KafkaConnectPage } from "@/pages/KafkaConnectPage";
import { ConnectorDetailPage } from "@/pages/ConnectorDetailPage";
import { KsqlPage } from "@/pages/KsqlPage";
import { AclPage } from "@/pages/AclPage";

const queryClient = new QueryClient({
  defaultOptions: { queries: { refetchOnWindowFocus: false, retry: 1 } },
});

export default function App() {
  return (
    <ThemeProvider defaultTheme="system">
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <Routes>
            <Route element={<Layout />}>
              <Route path="/" element={<DashboardPage />} />
              <Route path="/clusters" element={<ClustersPage />} />
              <Route path="/clusters/:clusterName/brokers" element={<BrokersPage />} />
              <Route path="/clusters/:clusterName/topics" element={<TopicsPage />} />
              <Route path="/clusters/:clusterName/topics/:topicName" element={<TopicDetailPage />} />
              <Route path="/clusters/:clusterName/topics/:topicName/messages" element={<TopicMessagesPage />} />
              <Route path="/clusters/:clusterName/consumer-groups" element={<ConsumerGroupsPage />} />
              <Route path="/clusters/:clusterName/consumer-groups/:groupName" element={<ConsumerGroupDetailPage />} />
              <Route path="/clusters/:clusterName/schemas" element={<SchemaRegistryPage />} />
              <Route path="/clusters/:clusterName/schemas/:subject" element={<SchemaDetailPage />} />
              <Route path="/clusters/:clusterName/connect" element={<KafkaConnectPage />} />
              <Route path="/clusters/:clusterName/connect/:connectorName" element={<ConnectorDetailPage />} />
              <Route path="/clusters/:clusterName/ksql" element={<KsqlPage />} />
              <Route path="/clusters/:clusterName/acl" element={<AclPage />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
