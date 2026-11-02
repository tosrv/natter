ALTER TABLE users ADD COLUMN display_name text;
UPDATE users SET display_name = left(CASE WHEN kind = 'guest' THEN 'Guest ' || upper(substr(username, 7)) ELSE coalesce(username, 'User') END, 40);
UPDATE users SET display_name = 'User' WHERE display_name IS NULL OR display_name = '';
ALTER TABLE users ALTER COLUMN display_name SET NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_display_name_len CHECK (char_length(display_name) BETWEEN 1 AND 40);
