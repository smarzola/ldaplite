-- Remove userPassword from attributes table for security
-- Passwords are stored securely in users.password_hash only
DELETE FROM attributes WHERE LOWER(name) = 'userpassword';
