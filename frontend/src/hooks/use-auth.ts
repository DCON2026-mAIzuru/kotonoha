'use client';

import { useEffect, useMemo, useState } from 'react';
import type { Session } from '@supabase/supabase-js';
import { getSupabaseClient } from '@/lib/supabase';

export function useAuth() {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [configurationError, setConfigurationError] = useState('');

  const getClientOrThrow = () => {
    try {
      return getSupabaseClient();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Supabase初期化に失敗しました';
      setConfigurationError(message);
      throw new Error(message);
    }
  };

  useEffect(() => {
    let mounted = true;

    let client;
    try {
      client = getClientOrThrow();
    } catch {
      if (mounted) {
        setLoading(false);
      }
      return () => {
        mounted = false;
      };
    }

    client.auth
      .getSession()
      .then(({ data }) => {
        if (mounted) {
          setSession(data.session);
          setLoading(false);
        }
      })
      .catch(() => {
        if (mounted) {
          setLoading(false);
        }
      });

    const {
      data: { subscription },
    } = client.auth.onAuthStateChange((_event, newSession) => {
      setSession(newSession);
      setLoading(false);
    });

    return () => {
      mounted = false;
      subscription.unsubscribe();
    };
  }, []);

  const accessToken = session?.access_token || '';

  const api = useMemo(
    () => ({
      async signInWithPassword(email: string, password: string) {
        const client = getClientOrThrow();
        const { error } = await client.auth.signInWithPassword({ email, password });
        if (error) {
          throw error;
        }
      },
      async signUpWithPassword(email: string, password: string) {
        const client = getClientOrThrow();
        const { error } = await client.auth.signUp({ email, password });
        if (error) {
          throw error;
        }
      },
      async signInWithGoogle() {
        const client = getClientOrThrow();
        const { error } = await client.auth.signInWithOAuth({
          provider: 'google',
          options: {
            redirectTo: typeof window === 'undefined' ? undefined : window.location.origin,
          },
        });
        if (error) {
          throw error;
        }
      },
      async signOut() {
        const client = getClientOrThrow();
        const { error } = await client.auth.signOut();
        if (error) {
          throw error;
        }
      },
    }),
    [],
  );

  return {
    session,
    accessToken,
    loading,
    configurationError,
    ...api,
  };
}
