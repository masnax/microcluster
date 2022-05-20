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
  joiner_cert    TEXT NOT NULL,
  token  TEXT NOT NULL,
  UNIQUE (joiner_cert),
  UNIQUE (token)
);
`
