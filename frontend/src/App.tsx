import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@/components/ThemeProvider";
import { Layout } from "@/components/layout/Layout";
import { ClustersPage } from "@/pages/ClustersPage";
import { BrokersPage } from "@/pages/BrokersPage";
import { TopicsPage } from "@/pages/TopicsPage";
import { TopicDetailPage } from "@/pages/TopicDetailPage";
import { TopicMessagesPage } from "@/pages/TopicMessagesPage";
import { PlaceholderPage } from "@/pages/PlaceholderPage";

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
              <Route path="/" element={<ClustersPage />} />
              <Route path="/clusters/:clusterName/brokers" element={<BrokersPage />} />
              <Route path="/clusters/:clusterName/topics" element={<TopicsPage />} />
              <Route path="/clusters/:clusterName/topics/:topicName" element={<TopicDetailPage />} />
              <Route path="/clusters/:clusterName/topics/:topicName/messages" element={<TopicMessagesPage />} />
              <Route path="/clusters/:clusterName/consumer-groups" element={<PlaceholderPage title="Consumer Groups" />} />
              <Route path="/clusters/:clusterName/schemas" element={<PlaceholderPage title="Schema Registry" />} />
              <Route path="/clusters/:clusterName/connect" element={<PlaceholderPage title="Kafka Connect" />} />
              <Route path="/clusters/:clusterName/ksql" element={<PlaceholderPage title="KSQL" />} />
              <Route path="/clusters/:clusterName/acl" element={<PlaceholderPage title="ACL" />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
