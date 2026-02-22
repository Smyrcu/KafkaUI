import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ConsumerGroupsPage } from './ConsumerGroupsPage';

vi.mock('@/lib/api', () => ({
  api: {
    consumerGroups: {
      list: vi.fn(),
    },
  },
}));

import { api } from '@/lib/api';

function renderWithRoute() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/clusters/test-cluster/consumer-groups']}>
        <Routes>
          <Route path="/clusters/:clusterName/consumer-groups" element={<ConsumerGroupsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe('ConsumerGroupsPage', () => {
  it('shows loading state', () => {
    (api.consumerGroups.list as ReturnType<typeof vi.fn>).mockReturnValue(new Promise(() => {}));
    renderWithRoute();
    expect(screen.getByRole('heading', { name: 'Consumer Groups' })).toBeInTheDocument();
  });

  it('renders consumer groups table', async () => {
    (api.consumerGroups.list as ReturnType<typeof vi.fn>).mockResolvedValue([
      { name: 'my-group', state: 'Stable', members: 3, topics: 2, coordinatorId: 0 },
      { name: 'other-group', state: 'Empty', members: 0, topics: 1, coordinatorId: 1 },
    ]);
    renderWithRoute();
    expect(await screen.findByText('my-group')).toBeInTheDocument();
    expect(await screen.findByText('other-group')).toBeInTheDocument();
    expect(await screen.findByText('Stable')).toBeInTheDocument();
    expect(await screen.findByText('Empty')).toBeInTheDocument();
  });

  it('filters groups by search', async () => {
    (api.consumerGroups.list as ReturnType<typeof vi.fn>).mockResolvedValue([
      { name: 'my-group', state: 'Stable', members: 3, topics: 2, coordinatorId: 0 },
      { name: 'other-group', state: 'Empty', members: 0, topics: 1, coordinatorId: 1 },
    ]);
    renderWithRoute();
    await screen.findByText('my-group');

    const searchInput = screen.getByPlaceholderText(/search/i);
    await userEvent.type(searchInput, 'other');

    expect(screen.queryByText('my-group')).not.toBeInTheDocument();
    expect(screen.getByText('other-group')).toBeInTheDocument();
  });

  it('shows empty state', async () => {
    (api.consumerGroups.list as ReturnType<typeof vi.fn>).mockResolvedValue([]);
    renderWithRoute();
    expect(await screen.findByText(/no consumer groups found/i)).toBeInTheDocument();
  });

  it('shows error on failure', async () => {
    (api.consumerGroups.list as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Cluster unreachable'));
    renderWithRoute();
    expect(await screen.findByText(/cluster unreachable/i)).toBeInTheDocument();
  });
});
