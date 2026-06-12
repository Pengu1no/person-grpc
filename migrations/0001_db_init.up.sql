CREATE TABLE persons (
    id UUID PRIMARY KEY,
    first_name VARCHAR(64) NOT NULL,
    last_name VARCHAR(64) NOT NULL,
    patronymic VARCHAR(64),
    age INT NOT NULL DEFAULT 0,
    gender INT NOT NULL DEFAULT 0,
    nationality VARCHAR(8) NOT NULL
);

CREATE TABLE person_emails (
    id BIGSERIAL PRIMARY KEY,
    person_id UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    email VARCHAR(128) NOT NULL,
    UNIQUE (person_id, email)
);

CREATE INDEX idx_persons_full_name ON persons (last_name, first_name);
CREATE INDEX idx_person_emails_person_id ON person_emails(person_id);