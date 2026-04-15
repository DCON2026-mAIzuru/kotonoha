import { createClient } from '@supabase/supabase-js';

let cachedClient: ReturnType<typeof createClient> | null = null;

export function getSupabaseClient() {
  if (cachedClient) {
    return cachedClient;
  }

  const supabaseUrl = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const supabaseAnonKey = process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY;

  if (!supabaseUrl || !supabaseAnonKey) {
    throw new Error('Supabaseの環境変数が不足しています');
  }

  cachedClient = createClient(supabaseUrl, supabaseAnonKey);
  return cachedClient;
}
