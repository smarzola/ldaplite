CREATE INDEX IF NOT EXISTS idx_attributes_lower_name_value_entry
ON attributes(LOWER(name), LOWER(value), entry_id);
