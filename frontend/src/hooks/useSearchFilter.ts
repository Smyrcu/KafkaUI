import { useState, useMemo } from 'react';

export function useSearchFilter<T>(items: T[], accessor: (item: T) => string) {
  const [search, setSearch] = useState("");
  const filtered = useMemo(
    () => items.filter((item) => accessor(item).toLowerCase().includes(search.toLowerCase())),
    [items, search, accessor]
  );
  return { search, setSearch, filtered };
}
