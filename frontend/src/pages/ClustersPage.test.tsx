import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { ClustersPage } from './ClustersPage';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    clusters: {
      list: vi.fn(),
    },
  },
}));

import { api } from '@/lib/api';

function renderWithProviders(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  );
}

describe('ClustersPage', () => {
  it('shows loading state', () => {
    (api.clusters.list as ReturnType<typeof vi.fn>).mockReturnValue(new Promise(() => {}));
    renderWithProviders(<ClustersPage />);
    expect(screen.getByRole('heading', { name: 'Clusters' })).toBeInTheDocument();
  });

  it('renders cluster cards', async () => {
    (api.clusters.list as ReturnType<typeof vi.fn>).mockResolvedValue([
      { name: 'production', bootstrapServers: 'kafka:9092' },
      { name: 'staging', bootstrapServers: 'kafka-staging:9092' },
    ]);
    renderWithProviders(<ClustersPage />);
    expect(await screen.findByText('production')).toBeInTheDocument();
    expect(await screen.findByText('staging')).toBeInTheDocument();
  });

  it('shows error on failure', async () => {
    (api.clusters.list as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Connection failed'));
    renderWithProviders(<ClustersPage />);
    expect(await screen.findByText(/connection failed/i)).toBeInTheDocument();
  });
});
