-- 盆栽会話システムの初期スキーマです。
-- Step 1 では Auth, プロフィール, 最新センサー状態, 会話履歴, Push購読, 長期記憶の土台を定義します。

create extension if not exists vector;
create extension if not exists pgcrypto;

create or replace function public.set_updated_at()
returns trigger
language plpgsql
as $$
begin
  new.updated_at = timezone('utc', now());
  return new;
end;
$$;

create table if not exists public.bonsai_profiles (
  user_id uuid primary key references auth.users (id) on delete cascade,
  name text not null check (char_length(name) between 1 and 50),
  personality text not null check (char_length(personality) between 1 and 500),
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

comment on table public.bonsai_profiles is '盆栽の表示名と性格設定を保持するテーブル';

create table if not exists public.push_subscriptions (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users (id) on delete cascade,
  endpoint text not null,
  subscription jsonb not null,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  unique (user_id, endpoint)
);

comment on table public.push_subscriptions is 'Web Push の購読情報を保持するテーブル';

create table if not exists public.sensor_data (
  user_id uuid primary key references auth.users (id) on delete cascade,
  device_id text not null,
  moisture_percent numeric(5, 2) not null check (moisture_percent >= 0 and moisture_percent <= 100),
  light_lux numeric(10, 2) not null check (light_lux >= 0),
  temperature_celsius numeric(5, 2) not null check (temperature_celsius >= -50 and temperature_celsius <= 100),
  recorded_at timestamptz not null,
  sensor_status text not null default 'normal' check (sensor_status in ('normal', 'stale', 'invalid')),
  raw_payload jsonb,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

comment on table public.sensor_data is 'ユーザーごとの最新センサー状態を保持するテーブル';

create table if not exists public.chat_history (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users (id) on delete cascade,
  role text not null check (role in ('user', 'assistant', 'system', 'event')),
  content text not null,
  sensor_snapshot jsonb,
  created_at timestamptz not null default timezone('utc', now())
);

comment on table public.chat_history is '短期記憶として使う会話履歴テーブル';

create table if not exists public.bonsai_memories (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users (id) on delete cascade,
  summary text not null,
  source_message_ids uuid[] not null default '{}',
  embedding vector(768) not null,
  created_at timestamptz not null default timezone('utc', now())
);

comment on table public.bonsai_memories is '長期記憶として使うベクトル化済みメモリテーブル';

create index if not exists idx_push_subscriptions_user_id
  on public.push_subscriptions (user_id);

create index if not exists idx_chat_history_user_id_created_at
  on public.chat_history (user_id, created_at desc);

create index if not exists idx_bonsai_memories_user_id_created_at
  on public.bonsai_memories (user_id, created_at desc);

create index if not exists idx_bonsai_memories_embedding
  on public.bonsai_memories
  using ivfflat (embedding vector_cosine_ops)
  with (lists = 100);

create trigger set_bonsai_profiles_updated_at
before update on public.bonsai_profiles
for each row
execute function public.set_updated_at();

create trigger set_push_subscriptions_updated_at
before update on public.push_subscriptions
for each row
execute function public.set_updated_at();

create trigger set_sensor_data_updated_at
before update on public.sensor_data
for each row
execute function public.set_updated_at();

alter table public.bonsai_profiles enable row level security;
alter table public.push_subscriptions enable row level security;
alter table public.sensor_data enable row level security;
alter table public.chat_history enable row level security;
alter table public.bonsai_memories enable row level security;

create policy "bonsai_profiles_select_own"
on public.bonsai_profiles
for select
using (auth.uid() = user_id);

create policy "bonsai_profiles_insert_own"
on public.bonsai_profiles
for insert
with check (auth.uid() = user_id);

create policy "bonsai_profiles_update_own"
on public.bonsai_profiles
for update
using (auth.uid() = user_id)
with check (auth.uid() = user_id);

create policy "bonsai_profiles_delete_own"
on public.bonsai_profiles
for delete
using (auth.uid() = user_id);

create policy "push_subscriptions_select_own"
on public.push_subscriptions
for select
using (auth.uid() = user_id);

create policy "push_subscriptions_insert_own"
on public.push_subscriptions
for insert
with check (auth.uid() = user_id);

create policy "push_subscriptions_update_own"
on public.push_subscriptions
for update
using (auth.uid() = user_id)
with check (auth.uid() = user_id);

create policy "push_subscriptions_delete_own"
on public.push_subscriptions
for delete
using (auth.uid() = user_id);

create policy "sensor_data_select_own"
on public.sensor_data
for select
using (auth.uid() = user_id);

create policy "sensor_data_insert_own"
on public.sensor_data
for insert
with check (auth.uid() = user_id);

create policy "sensor_data_update_own"
on public.sensor_data
for update
using (auth.uid() = user_id)
with check (auth.uid() = user_id);

create policy "sensor_data_delete_own"
on public.sensor_data
for delete
using (auth.uid() = user_id);

create policy "chat_history_select_own"
on public.chat_history
for select
using (auth.uid() = user_id);

create policy "chat_history_insert_own"
on public.chat_history
for insert
with check (auth.uid() = user_id);

create policy "chat_history_update_own"
on public.chat_history
for update
using (auth.uid() = user_id)
with check (auth.uid() = user_id);

create policy "chat_history_delete_own"
on public.chat_history
for delete
using (auth.uid() = user_id);

create policy "bonsai_memories_select_own"
on public.bonsai_memories
for select
using (auth.uid() = user_id);

create policy "bonsai_memories_insert_own"
on public.bonsai_memories
for insert
with check (auth.uid() = user_id);

create policy "bonsai_memories_update_own"
on public.bonsai_memories
for update
using (auth.uid() = user_id)
with check (auth.uid() = user_id);

create policy "bonsai_memories_delete_own"
on public.bonsai_memories
for delete
using (auth.uid() = user_id);

create or replace function public.match_bonsai_memories(
  query_embedding vector(768),
  match_count integer default 3
)
returns table (
  id uuid,
  summary text,
  similarity double precision,
  created_at timestamptz
)
language sql
stable
as $$
  select
    memory.id,
    memory.summary,
    1 - (memory.embedding <=> query_embedding) as similarity,
    memory.created_at
  from public.bonsai_memories as memory
  where memory.user_id = auth.uid()
  order by memory.embedding <=> query_embedding
  limit greatest(match_count, 1);
$$;
