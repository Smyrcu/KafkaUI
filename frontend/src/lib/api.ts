const API_BASE = '/api/v1';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    if (res.status === 401 && !path.startsWith('/auth/login')) {
      window.location.reload();
      throw new Error('Unauthorized');
    }
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

export interface MessageRecord {
  partition: number;
  offset: number;
  timestamp: string;
  key: string;
  value: string;
  headers?: Record<string, string>;
}

export interface ProduceRequest {
  key: string;
  value: string;
  partition?: number | null;
  headers?: Record<string, string>;
}

export interface BrowseParams {
  partition?: number;
  offset?: string;
  limit?: number;
  timestamp?: string;
  filter?: string;
}

export interface ConsumerGroupInfo {
  name: string;
  state: string;
  members: number;
  topics: number;
  coordinatorId: number;
}

export interface ConsumerGroupDetail {
  name: string;
  state: string;
  coordinatorId: number;
  members: ConsumerGroupMember[];
  offsets: ConsumerGroupTopicOffset[];
}

export interface ConsumerGroupMember {
  id: string;
  clientId: string;
  host: string;
  topics: string[];
}

export interface ConsumerGroupTopicOffset {
  topic: string;
  partitions: ConsumerGroupPartitionOffset[];
  totalLag: number;
}

export interface ConsumerGroupPartitionOffset {
  partition: number;
  currentOffset: number;
  endOffset: number;
  lag: number;
}

export interface ResetOffsetsRequest {
  topic: string;
  resetTo: string;
}

export interface ClusterOverview {
  name: string;
  bootstrapServers: string;
  brokerCount: number;
  topicCount: number;
  consumerGroupCount: number;
  status: string;
}

export interface MetricSample {
  labels?: Record<string, string>;
  value: number;
}

export interface MetricHistoryPoint {
  time: string;
  value: number;
}

export interface MetricDetail {
  name: string;
  help: string;
  type: string;
  current: MetricSample[];
  history: MetricHistoryPoint[];
}

export interface MetricGroup {
  name: string;
  prefix: string;
  metrics: MetricDetail[];
}

export interface MetricsResponse {
  groups: MetricGroup[];
}

export interface SchemaSubjectInfo {
  subject: string;
  latestVersion: number;
  latestSchemaId: number;
  schemaType: string;
}

export interface SchemaDetail {
  subject: string;
  compatibility: string;
  versions: SchemaVersion[];
}

export interface SchemaVersion {
  version: number;
  id: number;
  schema: string;
  schemaType: string;
}

export interface CreateSchemaRequest {
  subject: string;
  schema: string;
  schemaType: string;
}

export interface ConnectorInfo {
  name: string;
  type: string;
  state: string;
  workerId: string;
  connectCluster: string;
}

export interface ConnectorDetail {
  name: string;
  type: string;
  state: string;
  workerId: string;
  config: Record<string, string>;
  tasks: TaskStatus[];
  connectCluster: string;
}

export interface TaskStatus {
  id: number;
  state: string;
  workerId: string;
  trace?: string;
}

export interface CreateConnectorRequest {
  name: string;
  config: Record<string, string>;
}

export interface KsqlRequest {
  query: string;
}

export interface KsqlResponse {
  type: string;
  statementText?: string;
  warnings?: { message: string }[];
  data: unknown;
}

export interface ACLEntry {
  resourceType: string;
  resourceName: string;
  patternType: string;
  principal: string;
  host: string;
  operation: string;
  permission: string;
}

export interface ScramUser {
  name: string;
  mechanism: string;
  iterations: number;
}

export interface UpsertScramUserRequest {
  name: string;
  password: string;
  mechanism: string;
  iterations?: number;
}

export interface DeleteScramUserRequest {
  name: string;
  mechanism: string;
}

export interface OIDCProviderInfo {
  name: string;
  displayName?: string;
}

export interface AuthStatus {
  enabled: boolean;
  types: string[];
  providers?: OIDCProviderInfo[];
}

export interface AuthUser {
  authenticated: boolean;
  email?: string;
  name?: string;
  roles?: string[];
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface AdminUser {
  id: string;
  providerName: string;
  externalId: string;
  email: string;
  name: string;
  avatarUrl: string;
  roles: string[];
  lastLogin: string;
  createdAt: string;
}

export interface PermissionsResponse {
  actions: string[];
  clusters: string[];
}

export interface SetRolesRequest {
  roles: string[];
}

export interface AdminClusterInfo {
  name: string;
  bootstrapServers: string;
  dynamic: boolean;
}

export interface AdminClusterList {
  static: AdminClusterInfo[];
  dynamic: AdminClusterInfo[];
}

export interface AddClusterRequest {
  name: string;
  bootstrapServers: string;
  tls?: { enabled: boolean; caFile?: string };
  sasl?: { mechanism: string; username: string; password: string };
  schemaRegistry?: { url: string };
  kafkaConnect?: { name: string; url: string }[];
  ksql?: { url: string };
  metrics?: { url: string };
}

export interface TestConnectionResult {
  status: string;
  error?: string;
}

export const api = {
  dashboard: { overview: () => request<ClusterOverview[]>('/dashboard') },
  clusters: { list: () => request<ClusterInfo[]>('/clusters') },
  brokers: { list: (cluster: string) => request<BrokerInfo[]>(`/clusters/${cluster}/brokers`) },
  topics: {
    list: (cluster: string) => request<TopicInfo[]>(`/clusters/${cluster}/topics`),
    details: (cluster: string, topic: string) => request<TopicDetail>(`/clusters/${cluster}/topics/${topic}`),
    create: (cluster: string, data: CreateTopicRequest) => request(`/clusters/${cluster}/topics`, { method: 'POST', body: JSON.stringify(data) }),
    delete: (cluster: string, topic: string) => request(`/clusters/${cluster}/topics/${topic}`, { method: 'DELETE' }),
  },
  messages: {
    browse: (cluster: string, topic: string, params?: BrowseParams) => {
      const searchParams = new URLSearchParams();
      if (params?.partition !== undefined) searchParams.set('partition', String(params.partition));
      if (params?.offset !== undefined && params?.offset !== '') searchParams.set('offset', params.offset);
      if (params?.limit !== undefined) searchParams.set('limit', String(params.limit));
      if (params?.timestamp) searchParams.set('timestamp', params.timestamp);
      if (params?.filter) searchParams.set('filter', params.filter);
      const qs = searchParams.toString();
      return request<MessageRecord[]>(`/clusters/${cluster}/topics/${topic}/messages${qs ? `?${qs}` : ''}`);
    },
    produce: (cluster: string, topic: string, data: ProduceRequest) =>
      request<MessageRecord>(`/clusters/${cluster}/topics/${topic}/messages`, { method: 'POST', body: JSON.stringify(data) }),
  },
  consumerGroups: {
    list: (cluster: string) => request<ConsumerGroupInfo[]>(`/clusters/${cluster}/consumer-groups`),
    details: (cluster: string, group: string) => request<ConsumerGroupDetail>(`/clusters/${cluster}/consumer-groups/${encodeURIComponent(group)}`),
    resetOffsets: (cluster: string, group: string, data: ResetOffsetsRequest) =>
      request(`/clusters/${cluster}/consumer-groups/${encodeURIComponent(group)}/reset`, { method: 'POST', body: JSON.stringify(data) }),
  },
  schemas: {
    list: (cluster: string) => request<SchemaSubjectInfo[]>(`/clusters/${cluster}/schemas`),
    details: (cluster: string, subject: string) => request<SchemaDetail>(`/clusters/${cluster}/schemas/${encodeURIComponent(subject)}`),
    create: (cluster: string, data: CreateSchemaRequest) => request<{ id: number }>(`/clusters/${cluster}/schemas`, { method: 'POST', body: JSON.stringify(data) }),
    delete: (cluster: string, subject: string) => request(`/clusters/${cluster}/schemas/${encodeURIComponent(subject)}`, { method: 'DELETE' }),
  },
  connect: {
    list: (cluster: string) => request<ConnectorInfo[]>(`/clusters/${cluster}/connectors`),
    details: (cluster: string, name: string) => request<ConnectorDetail>(`/clusters/${cluster}/connectors/${encodeURIComponent(name)}`),
    create: (cluster: string, data: CreateConnectorRequest) => request<ConnectorDetail>(`/clusters/${cluster}/connectors`, { method: 'POST', body: JSON.stringify(data) }),
    update: (cluster: string, name: string, config: Record<string, string>) => request<ConnectorDetail>(`/clusters/${cluster}/connectors/${encodeURIComponent(name)}`, { method: 'PUT', body: JSON.stringify(config) }),
    delete: (cluster: string, name: string) => request(`/clusters/${cluster}/connectors/${encodeURIComponent(name)}`, { method: 'DELETE' }),
    restart: (cluster: string, name: string) => request(`/clusters/${cluster}/connectors/${encodeURIComponent(name)}/restart`, { method: 'POST' }),
    pause: (cluster: string, name: string) => request(`/clusters/${cluster}/connectors/${encodeURIComponent(name)}/pause`, { method: 'POST' }),
    resume: (cluster: string, name: string) => request(`/clusters/${cluster}/connectors/${encodeURIComponent(name)}/resume`, { method: 'POST' }),
  },
  ksql: {
    execute: (cluster: string, data: KsqlRequest) => request<KsqlResponse>(`/clusters/${cluster}/ksql`, { method: 'POST', body: JSON.stringify(data) }),
    info: (cluster: string) => request<Record<string, unknown>>(`/clusters/${cluster}/ksql/info`),
  },
  acl: {
    list: (cluster: string) => request<ACLEntry[]>(`/clusters/${cluster}/acls`),
    create: (cluster: string, data: ACLEntry) => request(`/clusters/${cluster}/acls`, { method: 'POST', body: JSON.stringify(data) }),
    delete: (cluster: string, data: ACLEntry) => request(`/clusters/${cluster}/acls/delete`, { method: 'POST', body: JSON.stringify(data) }),
  },
  users: {
    list: (cluster: string) => request<ScramUser[]>(`/clusters/${cluster}/users`),
    create: (cluster: string, data: UpsertScramUserRequest) =>
      request<{ status: string }>(`/clusters/${cluster}/users`, { method: 'POST', body: JSON.stringify(data) }),
    delete: (cluster: string, data: DeleteScramUserRequest) =>
      request<{ status: string }>(`/clusters/${cluster}/users/delete`, { method: 'POST', body: JSON.stringify(data) }),
  },
  metrics: {
    get: (cluster: string, range?: string, from?: string, to?: string) => {
      const params = new URLSearchParams();
      if (from) {
        params.set('from', from);
        if (to) params.set('to', to);
      } else if (range) {
        params.set('range', range);
      }
      const qs = params.toString();
      return request<MetricsResponse>(`/clusters/${cluster}/metrics${qs ? `?${qs}` : ''}`);
    },
  },
  auth: {
    status: () => request<AuthStatus>('/auth/status'),
    me: () => request<AuthUser>('/auth/me'),
    login: (data: LoginRequest) => request<AuthUser>('/auth/login', { method: 'POST', body: JSON.stringify(data) }),
    logout: () => request<{ status: string }>('/auth/logout', { method: 'POST' }),
    permissions: () => request<PermissionsResponse>('/auth/permissions'),
  },
  admin: {
    listClusters: () => request<AdminClusterList>('/admin/clusters'),
    addCluster: (data: AddClusterRequest, validate = true) =>
      request<{ status: string }>(`/admin/clusters${validate ? '' : '?validate=false'}`, { method: 'POST', body: JSON.stringify(data) }),
    updateCluster: (name: string, data: AddClusterRequest, validate = true) =>
      request<{ status: string }>(`/admin/clusters/${encodeURIComponent(name)}${validate ? '' : '?validate=false'}`, { method: 'PUT', body: JSON.stringify(data) }),
    deleteCluster: (name: string) =>
      request<{ status: string }>(`/admin/clusters/${encodeURIComponent(name)}`, { method: 'DELETE' }),
    testConnection: (data: AddClusterRequest) =>
      request<TestConnectionResult>('/admin/clusters/test', { method: 'POST', body: JSON.stringify(data) }),
    listUsers: () => request<AdminUser[]>('/admin/users'),
    getUser: (id: string) => request<AdminUser>(`/admin/users/${id}`),
    setUserRoles: (id: string, data: SetRolesRequest) =>
      request<{ status: string }>(`/admin/users/${id}/roles`, { method: 'PUT', body: JSON.stringify(data) }),
    deleteUser: (id: string) =>
      request<{ status: string }>(`/admin/users/${id}`, { method: 'DELETE' }),
  },
};
