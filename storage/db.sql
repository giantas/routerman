-- query: InitDb
BEGIN;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS devices;
CREATE TABLE users(
    id INTEGER NOT NULL PRIMARY KEY,
    name TEXT NOT NULL
);
CREATE TABLE devices(
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    alias TEXT DEFAULT "" NOT NULL,
    mac TEXT NOT NULL
);
COMMIT;

-- query: CreateUser
INSERT INTO users(name) VALUES($1) RETURNING id

-- query: GetUserById
SELECT id, name FROM users WHERE id = $1

-- query: GetUsers
SELECT id, name FROM users ORDER BY name ASC LIMIT $1 OFFSET $2

-- query: UpdateUser
UPDATE users SET name = $2 WHERE id = $1

-- query: DeleteUserById
DELETE FROM users WHERE id = $1

-- query: CreateDevice
INSERT INTO devices(user_id, alias, mac) VALUES($1, $2, $3) RETURNING id

-- query: GetDeviceById
SELECT id, user_id, alias, mac FROM devices WHERE id = $1

-- query: GetDevices
SELECT id, user_id, alias, mac FROM devices ORDER BY id DESC LIMIT $1 OFFSET $2

-- query: UpdateDevice
UPDATE devices SET user_id = $2, alias = $3, mac = $4 WHERE id = $1

-- query: DeleteDeviceById
DELETE FROM devices WHERE id = $1