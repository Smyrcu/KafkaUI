const API_BASE = '/api/v1';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(error.error || res.statusText);
  }
  return res.json();
}

export interface ClusterInfo { name: string; bootstrapServers: string; }
export interface BrokerInfo { id: number; host: string; port: number; rack?: string; }
export interface TopicInfo { name: string; partitions: number; replicas: number; internal: boolean; }
export interface TopicDetail { name: string; partitions: PartitionInfo[]; configs: Record<string, string>; internal: boolean; }
export interface PartitionInfo { id: number; leader: number; replicas: number[]; isr: number[]; }
export interface CreateTopicRequest { name: string; partitions: number; replicas: number; }

export const api = {
  clusters: { list: () => request<ClusterInfo[]>('/clusters') },
  brokers: { list: (cluster: string) => request<BrokerInfo[]>(`/clusters/${cluster}/brokers`) },
  topics: {
    list: (cluster: string) => request<TopicInfo[]>(`/clusters/${cluster}/topics`),
    details: (cluster: string, topic: string) => request<TopicDetail>(`/clusters/${cluster}/topics/${topic}`),
    create: (cluster: string, data: CreateTopicRequest) => request(`/clusters/${cluster}/topics`, { method: 'POST', body: JSON.stringify(data) }),
    delete: (cluster: string, topic: string) => request(`/clusters/${cluster}/topics/${topic}`, { method: 'DELETE' }),
  },
};
