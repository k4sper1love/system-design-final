CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    phone_number VARCHAR(20) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS balances (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    amount DECIMAL(15, 2) NOT NULL DEFAULT 0.0,
    version INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_user_balance UNIQUE (user_id)
);

CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    transaction_type VARCHAR(50) NOT NULL,
    user_id INT NOT NULL REFERENCES users(id),
    amount DECIMAL(15, 2) NOT NULL,
    recipient_id INT REFERENCES users(id),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    token VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_user_token UNIQUE (user_id, token)
);

CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_recipient_id ON transactions(recipient_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token ON refresh_tokens(token);

INSERT INTO users (phone_number, password_hash) 
VALUES 
    ('+79001234567', '$2a$10$NUIwJJj.zOcPKH.YX.OXUe5vNOK.BL0GZpCgKFn5TW8aXvBtkn.xW'), 
    ('+79009876543', '$2a$10$NUIwJJj.zOcPKH.YX.OXUe5vNOK.BL0GZpCgKFn5TW8aXvBtkn.xW') 
ON CONFLICT (phone_number) DO NOTHING;

INSERT INTO balances (user_id, amount)
SELECT id, 1000.00 FROM users WHERE phone_number = '+79001234567'
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO balances (user_id, amount)
SELECT id, 500.00 FROM users WHERE phone_number = '+79009876543'
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO transactions (transaction_type, user_id, amount, recipient_id, status)
SELECT 'top_up', u1.id, 1000.00, NULL, 'completed'
FROM users u1
WHERE u1.phone_number = '+79001234567'
LIMIT 1;

INSERT INTO transactions (transaction_type, user_id, amount, recipient_id, status)
SELECT 'top_up', u2.id, 500.00, NULL, 'completed'
FROM users u2
WHERE u2.phone_number = '+79009876543'
LIMIT 1;

INSERT INTO transactions (transaction_type, user_id, amount, recipient_id, status)
SELECT 'transfer', u1.id, 200.00, u2.id, 'completed'
FROM users u1, users u2
WHERE u1.phone_number = '+79001234567' AND u2.phone_number = '+79009876543'
LIMIT 1;