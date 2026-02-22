import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ErrorAlert } from './ErrorAlert';

describe('ErrorAlert', () => {
  it('renders the static heading', () => {
    render(<ErrorAlert message="Test error occurred" />);
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
  });

  it('renders the friendly error message', () => {
    render(<ErrorAlert message="Test error occurred" />);
    expect(screen.getByText('Test error occurred')).toBeInTheDocument();
  });

  it('transforms connection refused message', () => {
    render(<ErrorAlert message="connection refused" />);
    expect(screen.getByText(/unable to connect to the kafka broker/i)).toBeInTheDocument();
  });
});
