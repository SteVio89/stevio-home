import { useEffect, useState } from 'react';
import { adminListLocales, type AdminLocale } from '../api/client';

export function useAdminLocales() {
  const [locales, setLocales] = useState<AdminLocale[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  async function load() {
    try {
      const data = await adminListLocales();
      setLocales(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load locales');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  return { locales, loading, error, reload: load };
}
