CREATE TABLE users (
    id UUID PRIMARY KEY,
    address VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
