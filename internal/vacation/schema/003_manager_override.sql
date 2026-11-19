ALTER TABLE employee
    ADD COLUMN ad_manager_id UUID REFERENCES employee (id),
    ADD COLUMN manager_override_id UUID REFERENCES employee (id),
    ADD COLUMN manager_conflict_ad_id UUID REFERENCES employee (id),
    ADD COLUMN manager_conflict_raw TEXT NOT NULL DEFAULT '';

UPDATE employee
SET ad_manager_id = manager_id
WHERE manager_id IS NOT NULL
  AND ad_manager_id IS NULL;

CREATE INDEX employee_ad_manager_idx ON employee (ad_manager_id);
CREATE INDEX employee_manager_override_idx ON employee (manager_override_id);

ALTER TABLE notification ALTER COLUMN request_id DROP NOT NULL;
ALTER TABLE notification ADD COLUMN dedupe_key TEXT NOT NULL DEFAULT '';
ALTER TABLE notification ADD COLUMN payload JSONB NOT NULL DEFAULT '{}';

CREATE UNIQUE INDEX notification_manager_dedupe_idx
    ON notification (kind, recipient_employee_id, dedupe_key)
    WHERE dedupe_key <> '';
