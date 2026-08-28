DROP TRIGGER IF EXISTS trg_prevent_audit_modifications ON audit_entries;
DROP FUNCTION IF EXISTS prevent_audit_modifications();
DROP TABLE IF EXISTS audit_entries;
DROP TABLE IF EXISTS policies;
