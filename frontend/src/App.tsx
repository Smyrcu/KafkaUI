import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Layout } from "@/components/layout/Layout";
import { ClustersPage } from "@/pages/ClustersPage";
import { BrokersPage } from "@/pages/BrokersPage";
import { TopicsPage } from "@/pages/TopicsPage";
import { TopicDetailPage } from "@/pages/TopicDetailPage";

const queryClient = new QueryClient({
  defaultOptions: { queries: { refetchOnWindowFocus: false, retry: 1 } },
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<ClustersPage />} />
            <Route path="/clusters/:clusterName/brokers" element={<BrokersPage />} />
            <Route path="/clusters/:clusterName/topics" element={<TopicsPage />} />
            <Route path="/clusters/:clusterName/topics/:topicName" element={<TopicDetailPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
