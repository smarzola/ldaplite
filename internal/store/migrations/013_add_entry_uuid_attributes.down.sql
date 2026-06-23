DELETE FROM attributes
WHERE LOWER(name) IN ('entryuuid', 'uuid');
