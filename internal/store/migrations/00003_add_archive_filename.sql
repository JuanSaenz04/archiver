ALTER TABLE archives ADD COLUMN filename TEXT;
UPDATE archives SET filename = name;
