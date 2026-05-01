create table if not exists public.bonsai_diagnoses (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users (id) on delete cascade,
  health_score numeric(5, 2) not null check (health_score >= 0 and health_score <= 100),
  status text not null,
  status_label text not null,
  severity text not null,
  severity_label text not null,
  detail text not null,
  ratios jsonb not null,
  artifact_paths jsonb,
  raw_result jsonb,
  created_at timestamptz not null default timezone('utc', now())
);

comment on table public.bonsai_diagnoses is '画像診断による盆栽の健康状態を保持するテーブル';

create index if not exists idx_bonsai_diagnoses_user_created_at
  on public.bonsai_diagnoses (user_id, created_at desc);

alter table public.bonsai_diagnoses enable row level security;

create policy "bonsai_diagnoses_select_own"
on public.bonsai_diagnoses
for select
using (auth.uid() = user_id);

create policy "bonsai_diagnoses_insert_own"
on public.bonsai_diagnoses
for insert
with check (auth.uid() = user_id);

create policy "bonsai_diagnoses_update_own"
on public.bonsai_diagnoses
for update
using (auth.uid() = user_id)
with check (auth.uid() = user_id);

create policy "bonsai_diagnoses_delete_own"
on public.bonsai_diagnoses
for delete
using (auth.uid() = user_id);
