CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind text NOT NULL CHECK (kind IN ('google','guest')),
  google_sub text UNIQUE,
  email text,
  username text,
  username_changes int NOT NULL DEFAULT 0 CHECK (username_changes <= 3),
  is_developer bool NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_username_lower_key ON users (lower(username));
CREATE INDEX users_username_prefix_idx ON users (lower(username) text_pattern_ops);
CREATE INDEX users_kind_idx ON users (kind);

CREATE TABLE sessions (
  token_hash bytea PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE conversations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_a uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  user_b uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  CHECK (user_a < user_b),
  UNIQUE (user_a, user_b),
  last_seq bigint NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_activity_at timestamptz
);
CREATE INDEX conversations_last_activity_idx ON conversations (last_activity_at);
CREATE INDEX conversations_user_b_idx ON conversations (user_b);

CREATE TABLE conversation_members (
  conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  last_read_seq bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (conversation_id, user_id)
);
CREATE INDEX conversation_members_user_idx ON conversation_members (user_id);

CREATE TABLE messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  seq bigint NOT NULL,
  sender_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  client_msg_id uuid NOT NULL,
  body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (conversation_id, seq),
  UNIQUE (sender_id, client_msg_id)
);

CREATE TABLE user_slots (
  id int PRIMARY KEY CHECK (id = 1),
  used int NOT NULL CHECK (used BETWEEN 0 AND 300)
);
INSERT INTO user_slots VALUES (1, 0);
