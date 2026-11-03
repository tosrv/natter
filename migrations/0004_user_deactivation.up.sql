ALTER TABLE users ADD COLUMN deactivated_at timestamptz;
CREATE INDEX ON users (deactivated_at) WHERE deactivated_at IS NOT NULL;
