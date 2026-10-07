-- DESIGN EXAMPLE, not a migration. Requires tables described in architecture.md.
-- PostgreSQL placeholders for database/sql + pgx, NOT Bun NewRaw placeholders.
-- Inputs:
-- $1 viewer UUID, NULL for anonymous
-- $2 all_tag_ids UUID[] (unique, non-null elements)
-- $3 any_tag_ids UUID[] (unique, non-null elements)
-- $4 none_tag_ids UUID[] (unique, non-null elements)
-- $5 kinds TEXT[]; empty = all kinds
-- $6 scope TEXT: 'library' or 'discover' (validate enum before SQL)
-- $7 cursor_created_at TIMESTAMPTZ NULL on first page
-- $8 cursor_id UUID NULL on first page; validate both cursor fields together
-- $9 limit INT, validated 1..100; fetch one extra to derive next_cursor
-- Array parameters must be encoded using the pgx/database/sql compatible path.
-- This query uses tag IDs, not names. For discover by names across owners,
-- compile each normalized name to a separate EXISTS joining tags by name.
WITH p AS (
  SELECT $1::uuid AS viewer,
         COALESCE($2::uuid[], '{}'::uuid[]) AS all_ids,
         COALESCE($3::uuid[], '{}'::uuid[]) AS any_ids,
         COALESCE($4::uuid[], '{}'::uuid[]) AS none_ids,
         COALESCE($5::text[], '{}'::text[]) AS kinds,
         $6::text AS scope
)
SELECT n.id, n.owner_id, n.kind, n.visibility, n.title, n.created_at
FROM notes n CROSS JOIN p
WHERE n.deleted_at IS NULL
  AND (
    (p.scope = 'library' AND n.owner_id = p.viewer)
    OR
    (p.scope = 'discover' AND (
      n.visibility = 'public'
      OR (n.visibility = 'friends' AND (
        n.owner_id = p.viewer
        OR EXISTS (
          SELECT 1 FROM friendships f
          WHERE f.status = 'accepted'
            AND f.user_low = LEAST(n.owner_id, p.viewer)
            AND f.user_high = GREATEST(n.owner_id, p.viewer)
            AND p.viewer IS NOT NULL
        )
      ))
    ))
  )
  AND (cardinality(p.kinds) = 0 OR n.kind = ANY(p.kinds))
  AND NOT EXISTS (
    SELECT 1 FROM unnest(p.all_ids) AS required(tag_id)
    WHERE NOT EXISTS (
      SELECT 1 FROM note_tags nt
      WHERE nt.note_id = n.id AND nt.tag_id = required.tag_id
    )
  )
  AND (
    cardinality(p.any_ids) = 0
    OR EXISTS (
      SELECT 1 FROM note_tags nt
      WHERE nt.note_id = n.id AND nt.tag_id = ANY(p.any_ids)
    )
  )
  AND NOT EXISTS (
    SELECT 1 FROM note_tags nt
    WHERE nt.note_id = n.id AND nt.tag_id = ANY(p.none_ids)
  )
  AND (
    $7::timestamptz IS NULL
    OR (n.created_at, n.id) < ($7::timestamptz, $8::uuid)
  )
ORDER BY n.created_at DESC, n.id DESC
LIMIT ($9::integer + 1);
