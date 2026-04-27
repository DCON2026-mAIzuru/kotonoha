-- Step 12: 雑談・世話・なじみ具合チャット用の状態を追加する。

begin;

alter table public.bonsai_profiles
  add column if not exists tone_mode text not null default 'gentle',
  add column if not exists bond_score integer not null default 20,
  add column if not exists bond_updated_at timestamptz not null default timezone('utc', now());

do $$
begin
  if not exists (
    select 1
    from pg_constraint
    where conname = 'bonsai_profiles_tone_mode_check'
  ) then
    alter table public.bonsai_profiles
      add constraint bonsai_profiles_tone_mode_check
      check (tone_mode in ('gentle', 'childlike', 'calm', 'classical'));
  end if;

  if not exists (
    select 1
    from pg_constraint
    where conname = 'bonsai_profiles_bond_score_check'
  ) then
    alter table public.bonsai_profiles
      add constraint bonsai_profiles_bond_score_check
      check (bond_score >= 0 and bond_score <= 100);
  end if;
end
$$;

comment on column public.bonsai_profiles.tone_mode is
  '盆栽チャットの話し方モード';

comment on column public.bonsai_profiles.bond_score is
  '利用者と盆栽のなじみ具合。0から100で管理する';

comment on column public.bonsai_profiles.bond_updated_at is
  'なじみ具合の最終更新時刻';

commit;
