package cluster

const Schema = `
CREATE TABLE certificates (
  id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  fingerprint TEXT NOT NULL,
  type INTEGER NOT NULL,
  name TEXT NOT NULL,
  certificate text NOT NULL,
  UNIQUE (fingerprint)
);

CREATE TABLE secrets (
  id      INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
  certificate_id INTEGER NOT NULL,
  name    TEXT NOT NULL,
  secret  TEXT NOT NULL,
  UNIQUE (name, secret, certificate_id),
  FOREIGN KEY (certificate_id) REFERENCES certificates (id) ON DELETE CASCADE
);
`
