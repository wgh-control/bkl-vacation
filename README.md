# bkl-vacation

Go API and background worker for the BKL vacation portal. This repository holds the service, Postgres migrations, and the OpenAPI contract. The portal UI and the deploy manifests are separate.

The API covers vacation, unpaid and compensatory days off, and sick leave. Balances follow a working year that starts on the hire anniversary (Labor Code articles 121 and 122). The employee year view also returns planned, approved, and moved vacation days for past calendar years, the current year, and next year. An employee can plan a vacation in the next calendar year.

HR is an employee. A person in the HR group uses the employee routes for their own vacation, day off, and sick requests, and the HR routes for the approval queue, schedule, and reports. Reports export as PDF, XLS (SpreadsheetML), XLSX, or CSV, scoped to one person, one department, or everyone.

## Run

Postgres, Keycloak, and SMTP are external. Nothing in this repo stores a host name or a secret. Apply migrations, then start the API and the worker as two processes:

```sh
export DATABASE_URL=postgres://USER:PASSWORD@HOST:5432/bkl_vacation?sslmode=disable
go run ./cmd/migrate
export KEYCLOAK_ISSUER=https://HOST/realms/REALM
export KEYCLOAK_CLIENT_ID=vacation-portal
go run ./cmd/api
go run ./cmd/worker
```

The API listens on `:8080` (`HTTP_ADDR`). OpenAPI is `api/openapi.yaml` in the repo and `GET /api/openapi.yaml` on the server. Paths in the spec are relative to `/api`.

`go run ./cmd/api` requires `DATABASE_URL`, `KEYCLOAK_ISSUER`, and `KEYCLOAK_CLIENT_ID`. The worker requires `DATABASE_URL`. Directory sync runs only when `KEYCLOAK_ADMIN_CLIENT_ID` is set. Mail is sent only when `SMTP_HOST` and `SMTP_FROM` are set; a portal notice is still stored when mail fails. Each notice is a plain-text part plus an HTML part, in Russian or English, for the employee, the manager, or HR. When a vacation is approved, HR also receives that vacation by mail and can send it to finance for payment. The finance mailbox is `FINANCE_EMAIL`. It is not written in this repo.

| Variable | Role |
| --- | --- |
| `DATABASE_URL` | Postgres DSN |
| `HTTP_ADDR` | API listen address, default `:8080` |
| `KEYCLOAK_ISSUER` | Realm issuer URL. Tokens are checked with the realm JWKS |
| `KEYCLOAK_CLIENT_ID` | Expected `aud` or `azp` |
| `KEYCLOAK_CLIENT_SECRET` | Accepted, not used to validate tokens |
| `KEYCLOAK_HR_GROUP` | Group name that marks HR, default `hr` |
| `KEYCLOAK_ADMIN_CLIENT_ID` | Client-credentials client for the user sync |
| `KEYCLOAK_ADMIN_CLIENT_SECRET` | Secret for that client |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_TLS` | Mail server. `SMTP_TLS` is `starttls` (default), `tls`, or `off` |
| `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` | Mail credentials and From address |
| `FINANCE_EMAIL` | Mailbox that receives an approved vacation for payment. Empty until it is set |
| `CLAIM_EMAIL`, `CLAIM_LOCALE`, `CLAIM_EMPLOYEE_NUMBER`, `CLAIM_POSITION`, `CLAIM_DEPARTMENT`, `CLAIM_MANAGER`, `CLAIM_HIRE_DATE` | Token and directory attribute names |
| `SYNC_INTERVAL` | Directory sync period, default `15m` |
| `NOTIFY_INTERVAL` | Notice scan period, default `1m` |
| `CORS_ORIGIN` | Optional browser origin |

Directory data comes from Keycloak, which already reads Active Directory. The worker does not open its own LDAP connection. Sync does not delete people; a disabled account is marked inactive.

## Rules the API enforces

- The basic grant is 28 days plus an HR-set extra. Unpaid days off past 14 calendar days in a working year reduce accrual (article 121). The grant itself stays 28 plus extra.
- One vacation part must be at least 14 counted days, or enough free days must remain to form one. Holidays are not counted and do not split the part. Weekends count.
- The first six months of the first working year are capped at the days accrued so far, unless HR sets early leave.
- Vacation and day off can be filed for the current calendar year and the next one, including today, and not in the past. Sick leave can start up to a year ago.
- A range cannot cross a working-year boundary. The client splits it.
- Pay-by is a warning, not a block. The warning date is four days before the start; the statutory date (three days) is also returned.
- Confirmation opens five days before the start and stays open after the start. An unconfirmed vacation is not cancelled automatically.
- Sick leave may overlap an approved vacation. HR approval then needs a certificate number and a choice to extend or postpone the vacation.
- A reschedule repeats the manager step and then HR. The old dates stay reserved until HR approves the new range.

## Tests

```sh
go test ./...
```

The Postgres test is skipped unless `DATABASE_URL` is set. With a database it applies `migrations/001_init.sql` and runs one request through manager and HR approval.
