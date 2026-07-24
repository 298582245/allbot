DELETE FROM user_bind_codes
WHERE length(code) < 22;
