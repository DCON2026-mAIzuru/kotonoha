-- 既存環境向け: bonsai_memories の埋め込み次元を 768 -> 3072 へ移行する。
-- 768次元で保存された既存データは新旧モデル間で比較できないため、ここで再作成前に削除する。

begin;

truncate table public.bonsai_memories;

drop index if exists idx_bonsai_memories_embedding;

alter table public.bonsai_memories
  alter column embedding type vector(3072);

-- pgvector の ivfflat は vector 次元が 2000 以下でないと作成できないため、
-- 3072 次元ではインデックスを作らず逐次走査で検索する。

drop function if exists public.match_bonsai_memories(vector(768), integer);

create or replace function public.match_bonsai_memories(
  query_embedding vector(3072),
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

commit;
