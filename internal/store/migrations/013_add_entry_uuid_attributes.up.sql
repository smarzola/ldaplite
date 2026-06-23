CREATE TEMP TABLE temp_entry_uuids (
    entry_id INTEGER PRIMARY KEY,
    uuid TEXT NOT NULL
);

INSERT INTO temp_entry_uuids (entry_id, uuid)
SELECT
    e.id,
    COALESCE(
        (
            SELECT a.value
            FROM attributes a
            WHERE a.entry_id = e.id
              AND LOWER(a.name) = 'entryuuid'
            LIMIT 1
        ),
        (
            SELECT a.value
            FROM attributes a
            WHERE a.entry_id = e.id
              AND LOWER(a.name) = 'uuid'
            LIMIT 1
        ),
        LOWER(
            HEX(RANDOMBLOB(4)) || '-' ||
            HEX(RANDOMBLOB(2)) || '-' ||
            '4' || SUBSTR(HEX(RANDOMBLOB(2)), 2) || '-' ||
            SUBSTR('89ab', 1 + (ABS(RANDOM()) % 4), 1) || SUBSTR(HEX(RANDOMBLOB(2)), 2) || '-' ||
            HEX(RANDOMBLOB(6))
        )
    )
FROM entries e;

INSERT INTO attributes (entry_id, name, value)
SELECT t.entry_id, 'entryuuid', t.uuid
FROM temp_entry_uuids t
WHERE NOT EXISTS (
    SELECT 1
    FROM attributes a
    WHERE a.entry_id = t.entry_id
      AND LOWER(a.name) = 'entryuuid'
);

INSERT INTO attributes (entry_id, name, value)
SELECT t.entry_id, 'uuid', t.uuid
FROM temp_entry_uuids t
WHERE NOT EXISTS (
    SELECT 1
    FROM attributes a
    WHERE a.entry_id = t.entry_id
      AND LOWER(a.name) = 'uuid'
);

DROP TABLE temp_entry_uuids;
