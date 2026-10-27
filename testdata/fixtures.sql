-- Test Fixtures for Lethe Verification & Integration Tests

-- Clean slate for fixtures
TRUNCATE TABLE lethe_holds RESTART IDENTITY CASCADE;
TRUNCATE TABLE lethe_audit RESTART IDENTITY CASCADE;
TRUNCATE TABLE user_orders RESTART IDENTITY CASCADE;
TRUNCATE TABLE user_sessions RESTART IDENTITY CASCADE;
TRUNCATE TABLE users RESTART IDENTITY CASCADE;
TRUNCATE TABLE support_tickets RESTART IDENTITY CASCADE;

-- Insert users with varied ages and states
-- Assuming policy retention of 3y (cutoff = now() - 3y)

-- 1. Expired users (eligible for deletion)
INSERT INTO users (id, name, email, phone, kyc_verified, last_activity_at, anonymized_at, created_at)
VALUES
  (1, 'Alice Expired', 'alice@example.com', '1111111111', false, now() - INTERVAL '4 years', NULL, now() - INTERVAL '5 years'),
  (2, 'Bob Expired', 'bob@example.com', '2222222222', false, now() - INTERVAL '3 years 6 months', NULL, now() - INTERVAL '4 years'),
  (3, 'Charlie Expired', 'charlie@example.com', '3333333333', false, now() - INTERVAL '3 years 1 month', NULL, now() - INTERVAL '4 years');

-- 2. Unexpired users (must NOT be deleted)
INSERT INTO users (id, name, email, phone, kyc_verified, last_activity_at, anonymized_at, created_at)
VALUES
  (4, 'David Active', 'david@example.com', '4444444444', false, now() - INTERVAL '1 year', NULL, now() - INTERVAL '2 years'),
  (5, 'Eve Active', 'eve@example.com', '5555555555', false, now() - INTERVAL '1 month', NULL, now() - INTERVAL '6 months');

-- 3. Held users (expired activity, but protected by legal holds in lethe_holds)
INSERT INTO users (id, name, email, phone, kyc_verified, last_activity_at, anonymized_at, created_at)
VALUES
  (6, 'Frank Indefinite Hold', 'frank@example.com', '6666666666', false, now() - INTERVAL '4 years', NULL, now() - INTERVAL '5 years'),
  (7, 'Grace Future Hold', 'grace@example.com', '7777777777', false, now() - INTERVAL '4 years', NULL, now() - INTERVAL '5 years'),
  (8, 'Heidi Past Expired Hold', 'heidi@example.com', '8888888888', false, now() - INTERVAL '4 years', NULL, now() - INTERVAL '5 years'),
  (9, 'Ivan Released Hold', 'ivan@example.com', '9999999999', false, now() - INTERVAL '4 years', NULL, now() - INTERVAL '5 years');

-- 4. Protected by policy predicate hold_if: "kyc_verified = true"
INSERT INTO users (id, name, email, phone, kyc_verified, last_activity_at, anonymized_at, created_at)
VALUES
  (10, 'Judy KYC Protected', 'judy@example.com', '1010101010', true, now() - INTERVAL '4 years', NULL, now() - INTERVAL '5 years');

-- 5. Null trigger users (can never match '< cutoff'; should be flagged by plan)
INSERT INTO users (id, name, email, phone, kyc_verified, last_activity_at, anonymized_at, created_at)
VALUES
  (11, 'Kevin Null Activity', 'kevin@example.com', '1212121212', false, NULL, NULL, now() - INTERVAL '4 years'),
  (12, 'Laura Null Activity', 'laura@example.com', '1313131313', false, NULL, NULL, now() - INTERVAL '2 years');

-- 6. Already anonymized user (marker is set; should be excluded)
INSERT INTO users (id, name, email, phone, kyc_verified, last_activity_at, anonymized_at, created_at)
VALUES
  (13, 'Mallory Anonymized', 'redacted-13@invalid', NULL, false, now() - INTERVAL '4 years', now() - INTERVAL '6 months', now() - INTERVAL '5 years');

-- Synchronize sequence
SELECT setval('users_id_seq', 20, true);

-- Legal holds matching users 6, 7, 8, 9
INSERT INTO lethe_holds (id, entity, record_id, reason, basis, until, created_at, released_at)
VALUES
  (1, 'users', '6', 'Tax Audit 2024', 'Income Tax Act', NULL, now() - INTERVAL '1 year', NULL),                           -- Active indefinite
  (2, 'users', '7', 'Active Dispute #402', 'Litigation', now() + INTERVAL '1 year', now() - INTERVAL '6 months', NULL),     -- Active future
  (3, 'users', '8', 'Expired Investigation', 'PMLA', now() - INTERVAL '1 month', now() - INTERVAL '2 years', NULL),        -- Expired hold (should NOT protect)
  (4, 'users', '9', 'Settled Dispute', 'Court Order', NULL, now() - INTERVAL '1 year', now() - INTERVAL '1 day');           -- Released hold (should NOT protect)

SELECT setval('lethe_holds_id_seq', 10, true);

-- User sessions (Cascading child rows)
INSERT INTO user_sessions (user_id, session_token, created_at)
VALUES
  (1, 'sess_alice_1', now() - INTERVAL '4 years'),
  (1, 'sess_alice_2', now() - INTERVAL '4 years 1 day'),
  (2, 'sess_bob_1', now() - INTERVAL '3 years 6 months'),
  (4, 'sess_david_1', now() - INTERVAL '1 year'),
  (6, 'sess_frank_1', now() - INTERVAL '4 years');

-- Support tickets for Anonymization testing (assuming 1y retention)
INSERT INTO support_tickets (id, email, phone, name, details, closed_at, anonymized_at)
VALUES
  (1, 'ticket1@example.com', '555-001', 'Ticket One', 'Sensitive issue details', now() - INTERVAL '2 years', NULL),             -- Expired, eligible for anonymization
  (2, 'ticket2@example.com', '555-002', 'Ticket Two', 'Another issue', now() - INTERVAL '1 year 6 months', NULL),                 -- Expired, eligible for anonymization
  (3, 'ticket3@example.com', '555-003', 'Ticket Three', 'Recent ticket', now() - INTERVAL '2 months', NULL),                     -- Unexpired, keep intact
  (4, 'redacted-4@invalid', NULL, 'REDACTED', 'Redacted ticket', now() - INTERVAL '2 years', now() - INTERVAL '3 months');       -- Already anonymized
