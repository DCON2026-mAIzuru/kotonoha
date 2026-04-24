-- 盆栽プロフィールに、初期設定された来歴・過去の思い出を追加する。

begin;

alter table public.bonsai_profiles
  add column if not exists backstory text not null default '';

do $$
begin
  if not exists (
    select 1
    from pg_constraint
    where conname = 'bonsai_profiles_backstory_length'
  ) then
    alter table public.bonsai_profiles
      add constraint bonsai_profiles_backstory_length
      check (char_length(backstory) <= 2000);
  end if;
end
$$;

comment on column public.bonsai_profiles.backstory is
  '盆栽が最初から持っている来歴や前の持ち主との思い出';

commit;
