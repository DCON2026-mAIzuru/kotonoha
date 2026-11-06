begin;

-- 既存ベクトルのモデルは推測せず隔離する。履歴・記憶・ベクトル値は削除しない。
alter table public.bonsai_memories
  add column if not exists embedding_space text not null default 'legacy:unknown';

-- 新しい応答には実際の返信先を残す。旧履歴の関係は推測して書き換えない。
alter table public.chat_history
  add column if not exists reply_to_message_id uuid references public.chat_history(id) on delete set null;

create index if not exists idx_bonsai_memories_user_space
  on public.bonsai_memories (user_id, embedding_space);

-- 同一空間では1つの成功応答を1度だけ記憶する。既存の未知空間には制約を掛けない。
create unique index if not exists idx_bonsai_memories_space_assistant
  on public.bonsai_memories (user_id, embedding_space, (source_message_ids[2]))
  where embedding_space <> 'legacy:unknown' and cardinality(source_message_ids) = 2;

-- 旧RPCも新しいモデルのベクトルと混在させない。新backendは空間を指定して検索する。
create or replace function public.match_bonsai_memories(
  query_embedding vector(3072),
  match_count integer default 3
)
returns table (id uuid, summary text, similarity double precision, created_at timestamptz)
language sql
stable
as $$
  select memory.id, memory.summary,
    1 - (memory.embedding <=> query_embedding) as similarity,
    memory.created_at
  from public.bonsai_memories as memory
  where memory.user_id = auth.uid() and memory.embedding_space = 'legacy:unknown'
  order by memory.embedding <=> query_embedding
  limit greatest(match_count, 1);
$$;

commit;
