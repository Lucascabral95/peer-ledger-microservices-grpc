CREATE DATABASE users_db;

\c users_db;

CREATE TABLE users (
    id         VARCHAR(36)  PRIMARY KEY,
    name       VARCHAR(100) NOT NULL,
    email      VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMP    NOT NULL DEFAULT NOW()
);

INSERT INTO users (id, name, email) VALUES
    ('user-001', 'Lucas García',  'lucas@mail.com'),
    ('user-002', 'Ana López',     'ana@mail.com'),
    ('user-003', 'Marcos Díaz',   'marcos@mail.com');