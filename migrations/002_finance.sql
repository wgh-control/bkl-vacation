ALTER TABLE leave_request
    ADD COLUMN finance_sent_at TIMESTAMPTZ,
    ADD COLUMN finance_sent_by UUID REFERENCES employee (id);
