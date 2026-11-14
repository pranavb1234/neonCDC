-- Run this file in the Neon SQL Editor or against Neon using its direct connection string.

CREATE TABLE users (
  id serial PRIMARY KEY,
  name text NOT NULL,
  email text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE orders (
  id serial PRIMARY KEY,
  user_id integer NOT NULL,
  amount numeric(10,2) NOT NULL,
  status text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO users (name, email) VALUES
  ('Ada Lovelace', 'ada@example.com'),
  ('Grace Hopper', 'grace@example.com');

INSERT INTO orders (user_id, amount, status) VALUES
  (1, 49.99, 'pending'),
  (2, 125.50, 'paid');

ALTER TABLE users REPLICA IDENTITY FULL;
ALTER TABLE orders REPLICA IDENTITY FULL;
