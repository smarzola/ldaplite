CREATE INDEX IF NOT EXISTS idx_entries_lower_object_class
ON entries(LOWER(object_class));

CREATE INDEX IF NOT EXISTS idx_attributes_entry_lower_name
ON attributes(entry_id, LOWER(name));

CREATE INDEX IF NOT EXISTS idx_attributes_entry_lower_name_value
ON attributes(entry_id, LOWER(name), LOWER(value));
