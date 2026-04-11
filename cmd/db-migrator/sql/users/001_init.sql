CREATE TABLE IF NOT EXISTS users (
    id            VARCHAR(36)  PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    email         VARCHAR(100) NOT NULL UNIQUE,
    password_hash TEXT         NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO users (id, name, email, password_hash) VALUES
    ('user-001', 'Lucas Garcia',  'lucas@mail.com',  'pbkdf2_sha256$120000$Vv5CnUPNtYQJqJqTCELeTg$K+V8QY+ar1Ama1ynfZPRo4q6MMZcvwh0yYqoPsBbJdw'),
    ('user-002', 'Ana Lopez',     'ana@mail.com',    'pbkdf2_sha256$120000$EAS02NzQB5VT50I4ezGV9w$g03AQSdPXye7xWbfeaHTzEYxe8IorpPDcTKEYTOv5Sg'),
    ('user-003', 'Marcos Diaz',   'marcos@mail.com', 'pbkdf2_sha256$120000$/lyTQK9yJgAdWIwgwfDhkw$n88jdNu9b+38alVytp9/p2UkAEiMEOfk4P1g+zFWrIo')
ON CONFLICT (id) DO NOTHING;
