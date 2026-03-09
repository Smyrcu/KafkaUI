import { useState, useMemo } from 'react';

/**
 * Generic search filter hook. The accessor should be stable (e.g. wrapped
 * in useCallback) to avoid re-filtering on every render.
 */
export function useSearchFilter<T>(items: T[], accessor: (item: T) => string) {
  const [search, setSearch] = useState("");
  const filtered = useMemo(
    () => items.filter((item) => (accessor(item) ?? "").toLowerCase().includes(search.toLowerCase())),
    [items, search, accessor]
  );
  return { search, setSearch, filtered };
}
