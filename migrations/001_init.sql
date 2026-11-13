CREATE TABLE org_unit (
    id UUID PRIMARY KEY,
    external_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    parent_id UUID REFERENCES org_unit (id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE employee (
    id UUID PRIMARY KEY,
    keycloak_sub TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    locale TEXT NOT NULL DEFAULT 'ru',
    locale_chosen BOOLEAN NOT NULL DEFAULT FALSE,
    employee_number TEXT NOT NULL DEFAULT '',
    position TEXT NOT NULL DEFAULT '',
    department_id UUID REFERENCES org_unit (id),
    manager_id UUID REFERENCES employee (id),
    manager_raw TEXT NOT NULL DEFAULT '',
    hire_date DATE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    hr BOOLEAN NOT NULL DEFAULT FALSE,
    extra_days INTEGER NOT NULL DEFAULT 0,
    early_leave BOOLEAN NOT NULL DEFAULT FALSE,
    raw_attributes JSONB NOT NULL DEFAULT '{}',
    last_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX employee_manager_idx ON employee (manager_id);
CREATE INDEX employee_department_idx ON employee (department_id);

CREATE TABLE leave_year (
    id UUID PRIMARY KEY,
    employee_id UUID NOT NULL REFERENCES employee (id),
    starts_on DATE NOT NULL,
    ends_on DATE NOT NULL,
    granted_days INTEGER NOT NULL DEFAULT 28,
    extra_days INTEGER NOT NULL DEFAULT 0,
    accrued_adjustment DOUBLE PRECISION NOT NULL DEFAULT 0,
    unpaid_calendar_days INTEGER NOT NULL DEFAULT 0,
    excess_unpaid_days INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (employee_id, starts_on)
);

CREATE TABLE leave_request (
    id UUID PRIMARY KEY,
    employee_id UUID NOT NULL REFERENCES employee (id),
    leave_year_id UUID REFERENCES leave_year (id),
    type TEXT NOT NULL CHECK (type IN ('vacation', 'day_off', 'sick')),
    status TEXT NOT NULL CHECK (status IN (
        'draft', 'pending_manager', 'pending_hr', 'scheduled',
        'awaiting_confirmation', 'confirmed', 'processed',
        'rejected', 'cancelled'
    )),
    starts_on DATE NOT NULL,
    ends_on DATE NOT NULL,
    counted_days INTEGER NOT NULL CHECK (counted_days >= 0),
    reason_code TEXT NOT NULL DEFAULT '',
    certificate_number TEXT NOT NULL DEFAULT '',
    replaces_id UUID REFERENCES leave_request (id),
    successor_id UUID REFERENCES leave_request (id),
    cancel_reason TEXT NOT NULL DEFAULT '',
    overlap_choice TEXT NOT NULL DEFAULT '',
    manager_vacant BOOLEAN NOT NULL DEFAULT FALSE,
    self_approval BOOLEAN NOT NULL DEFAULT FALSE,
    needs_confirmation BOOLEAN NOT NULL DEFAULT FALSE,
    pay_by_warning BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_at TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    processed_by UUID REFERENCES employee (id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CHECK (ends_on >= starts_on)
);

CREATE INDEX leave_request_employee_status_idx ON leave_request (employee_id, status);
CREATE INDEX leave_request_dates_idx ON leave_request (starts_on, ends_on);
CREATE INDEX leave_request_status_idx ON leave_request (status);

CREATE TABLE approval_step (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES leave_request (id),
    sequence INTEGER NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('manager', 'hr')),
    expected_employee_id UUID REFERENCES employee (id),
    decision TEXT NOT NULL DEFAULT '',
    comment TEXT NOT NULL DEFAULT '',
    decided_at TIMESTAMPTZ,
    actor_id UUID REFERENCES employee (id),
    UNIQUE (request_id, sequence)
);

CREATE INDEX approval_step_expected_idx ON approval_step (expected_employee_id);

CREATE TABLE calendar_day (
    day DATE PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('work', 'weekend', 'holiday', 'transferred')),
    name TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE notification (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES leave_request (id),
    recipient_employee_id UUID NOT NULL REFERENCES employee (id),
    kind TEXT NOT NULL,
    recipient_email TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT 'ru',
    due_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    notice_created BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (request_id, kind, recipient_employee_id)
);

CREATE INDEX notification_unsent_idx ON notification (due_at) WHERE sent_at IS NULL;

CREATE TABLE portal_notice (
    id UUID PRIMARY KEY,
    employee_id UUID NOT NULL REFERENCES employee (id),
    request_id UUID REFERENCES leave_request (id),
    notification_id UUID REFERENCES notification (id),
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    read_at TIMESTAMPTZ
);

CREATE INDEX portal_notice_employee_idx ON portal_notice (employee_id, created_at DESC);

CREATE TABLE audit_event (
    id UUID PRIMARY KEY,
    actor_id UUID,
    request_id UUID,
    employee_id UUID,
    action TEXT NOT NULL,
    before_status TEXT NOT NULL DEFAULT '',
    after_status TEXT NOT NULL DEFAULT '',
    detail JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX audit_event_request_idx ON audit_event (request_id, created_at);

CREATE TABLE schedule_lock (
    year INTEGER PRIMARY KEY,
    locked_by UUID REFERENCES employee (id),
    locked_at TIMESTAMPTZ NOT NULL
);
