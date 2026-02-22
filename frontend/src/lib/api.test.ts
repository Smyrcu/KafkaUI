import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from './api';

// Mock global fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

beforeEach(() => {
  mockFetch.mockReset();
});

function mockResponse(data: unknown, status = 200) {
  mockFetch.mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? 'OK' : 'Error',
    json: () => Promise.resolve(data),
  });
}

describe('api.clusters', () => {
  it('list fetches /clusters', async () => {
    const clusters = [{ name: 'local', bootstrapServers: 'localhost:9092' }];
    mockResponse(clusters);
    const result = await api.clusters.list();
    expect(result).toEqual(clusters);
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/clusters', expect.objectContaining({
      headers: { 'Content-Type': 'application/json' },
    }));
  });
});

describe('api.brokers', () => {
  it('list fetches /clusters/{name}/brokers', async () => {
    const brokers = [{ id: 0, host: 'localhost', port: 9092 }];
    mockResponse(brokers);
    const result = await api.brokers.list('test');
    expect(result).toEqual(brokers);
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/clusters/test/brokers', expect.anything());
  });
});

describe('api.topics', () => {
  it('list fetches topics', async () => {
    const topics = [{ name: 'test', partitions: 3, replicas: 1, internal: false }];
    mockResponse(topics);
    const result = await api.topics.list('cluster');
    expect(result).toEqual(topics);
  });

  it('details fetches topic detail', async () => {
    const detail = { name: 'test', partitions: [], configs: {}, internal: false };
    mockResponse(detail);
    const result = await api.topics.details('cluster', 'test');
    expect(result).toEqual(detail);
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/clusters/cluster/topics/test', expect.anything());
  });

  it('create sends POST', async () => {
    mockResponse({ status: 'created' }, 201);
    await api.topics.create('cluster', { name: 'new', partitions: 1, replicas: 1 });
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/clusters/cluster/topics', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: 'new', partitions: 1, replicas: 1 }),
    }));
  });

  it('delete sends DELETE', async () => {
    mockResponse({ status: 'deleted' });
    await api.topics.delete('cluster', 'test');
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/clusters/cluster/topics/test', expect.objectContaining({
      method: 'DELETE',
    }));
  });
});

describe('api.messages', () => {
  it('browse with params builds query string', async () => {
    mockResponse([]);
    await api.messages.browse('cluster', 'topic', { partition: 0, offset: 'earliest', limit: 50 });
    const url = mockFetch.mock.calls[0][0] as string;
    expect(url).toContain('partition=0');
    expect(url).toContain('offset=earliest');
    expect(url).toContain('limit=50');
  });

  it('browse without params has no query string', async () => {
    mockResponse([]);
    await api.messages.browse('cluster', 'topic');
    const url = mockFetch.mock.calls[0][0] as string;
    expect(url).toBe('/api/v1/clusters/cluster/topics/topic/messages');
  });

  it('produce sends POST with body', async () => {
    const record = { partition: 0, offset: 1, timestamp: '', key: 'k', value: 'v' };
    mockResponse(record);
    await api.messages.produce('cluster', 'topic', { key: 'k', value: 'v' });
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/v1/clusters/cluster/topics/topic/messages',
      expect.objectContaining({ method: 'POST' }),
    );
  });
});

describe('api.consumerGroups', () => {
  it('list fetches consumer groups', async () => {
    const groups = [{ name: 'group1', state: 'Stable', members: 1, topics: 2, coordinatorId: 0 }];
    mockResponse(groups);
    const result = await api.consumerGroups.list('cluster');
    expect(result).toEqual(groups);
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/clusters/cluster/consumer-groups', expect.anything());
  });

  it('details fetches consumer group detail', async () => {
    const detail = { name: 'group1', state: 'Stable', coordinatorId: 0, members: [], offsets: [] };
    mockResponse(detail);
    const result = await api.consumerGroups.details('cluster', 'group1');
    expect(result).toEqual(detail);
  });

  it('details encodes group name', async () => {
    mockResponse({});
    await api.consumerGroups.details('cluster', 'group/with/slashes');
    const url = mockFetch.mock.calls[0][0] as string;
    expect(url).toContain('group%2Fwith%2Fslashes');
  });

  it('resetOffsets sends POST', async () => {
    mockResponse({ status: 'ok' });
    await api.consumerGroups.resetOffsets('cluster', 'group1', { topic: 'test', resetTo: 'earliest' });
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining('/consumer-groups/group1/reset'),
      expect.objectContaining({ method: 'POST' }),
    );
  });
});

describe('error handling', () => {
  it('throws on non-ok response', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.resolve({ error: 'something broke' }),
    });
    await expect(api.clusters.list()).rejects.toThrow('something broke');
  });

  it('throws statusText when no error body', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.reject(new Error('no json')),
    });
    await expect(api.clusters.list()).rejects.toThrow('Internal Server Error');
  });
});
