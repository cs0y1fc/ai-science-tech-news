-- Favoritos de Tech News. Pégalo en Supabase → SQL Editor y ejecútalo una vez.
-- Cada usuario solo puede ver, crear y borrar sus propias filas (RLS).

create table if not exists public.favorites (
  user_id    uuid not null default auth.uid() references auth.users (id) on delete cascade,
  article_id text not null,
  title      text not null check (char_length(title) <= 500),
  url        text not null check (char_length(url) <= 2000),
  source     text not null default '',
  section    text not null default 'tech',
  image_url  text,
  published  timestamptz,
  created_at timestamptz not null default now(),
  primary key (user_id, article_id)
);

alter table public.favorites enable row level security;

create policy "own favorites: select" on public.favorites
  for select to authenticated using (user_id = auth.uid());
create policy "own favorites: insert" on public.favorites
  for insert to authenticated with check (user_id = auth.uid());
create policy "own favorites: delete" on public.favorites
  for delete to authenticated using (user_id = auth.uid());
