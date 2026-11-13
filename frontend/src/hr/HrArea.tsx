import { useCallback, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "../api/errors";
import type { LeaveRequest, RequestType } from "../api/types";
import { YearCalendar } from "../components/YearCalendar";
import { StatusPill } from "../components/StatusPill";
import { calendarYear, todayISO, yearBounds } from "../domain/dates";
import { formatDays, formatRange } from "../format";
import { translateError, useI18n } from "../i18n/context";
import { useApi, useMe } from "../session";
import { useLoad } from "../useLoad";
import { hrBlock } from "./rules";

const filters: Array<RequestType | "all"> = ["all", "vacation", "day_off", "sick"];

function QueueCard({
  request,
  onDecide,
}: {
  request: LeaveRequest;
  onDecide: (decision: "approve" | "reject", comment: string) => Promise<void>;
}) {
  const me = useMe();
  const { t, language } = useI18n();
  const [comment, setComment] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const block = hrBlock(request, me.id);

  async function act(decision: "approve" | "reject") {
    setPending(true);
    setError(null);
    try {
      if (block === "self") throw new ApiError("forbidden", 403);
      if (decision === "approve" && block === "certificate") throw new ApiError("certificate_required", 400);
      await onDecide(decision, comment);
    } catch (reason) {
      setError(reason instanceof ApiError ? reason.code : "unknown");
    } finally {
      setPending(false);
    }
  }

  return (
    <article className="card decision">
      <div className="item-top">
        <div>
          <strong>{request.person.displayName}</strong>
          <div className="muted">
            {request.person.department} · {t(`request.type.${request.type}`)}
          </div>
        </div>
        <StatusPill status={request.status} />
      </div>
      <p>
        <Link to={`/requests/${request.id}`}>{formatRange(request.start, request.end, language)}</Link>
        {" · "}
        {formatDays(language, request.countedDays)}
      </p>
      {request.managerDecision ? (
        <p className="muted">
          {t("hr.managerDecision")}: {request.managerDecision.decision === "approve" ? t("hr.approve") : t("hr.reject")}
          {request.managerDecision.comment ? ` — ${request.managerDecision.comment}` : ""}
        </p>
      ) : request.status === "pending_hr" ? (
        <p className="muted">{t("hr.noManagerDecision")}</p>
      ) : null}
      {request.type === "sick" ? (
        <p className="muted">
          {t("detail.certificate")}: {request.certificateNumber || "—"}
        </p>
      ) : null}
      {request.reasonCode ? <p className="muted">{t(`reason.${request.reasonCode}`)}</p> : null}
      {request.payByWarning && request.type === "vacation" ? <p className="banner banner-warn">{t("hr.payByWarning")}</p> : null}
      {request.routedToHrBecauseManagerVacant ? <p className="banner banner-info">{t("detail.managerVacant")}</p> : null}
      {block === "self" ? (
        <p className="banner banner-info">
          {t("hr.myArea")} <Link to="/">{t("nav.employee")}</Link>
        </p>
      ) : null}
      {block === "certificate" ? <p className="banner banner-warn">{t("hr.certificateMissing")}</p> : null}
      <label className="field">
        <span>
          {t("request.wizard.comment")} <span className="muted">({t("common.optional")})</span>
        </span>
        <textarea value={comment} onChange={(event) => setComment(event.target.value)} rows={2} disabled={block === "self"} />
      </label>
      {error ? <p className="banner banner-warn">{translateError(t, error)}</p> : null}
      <div className="actions">
        <button className="btn btn-primary" type="button" disabled={pending || block !== null} onClick={() => void act("approve")}>
          {t("hr.approve")}
        </button>
        <button className="btn btn-ghost" type="button" disabled={pending || block === "self"} onClick={() => void act("reject")}>
          {t("hr.reject")}
        </button>
      </div>
    </article>
  );
}

export function HrArea() {
  const api = useApi();
  const { t, language } = useI18n();
  const today = todayISO();
  const year = calendarYear(today);
  const { from, to } = yearBounds(year);
  const [filter, setFilter] = useState<RequestType | "all">("all");
  const [departmentId, setDepartmentId] = useState("all");
  const load = useCallback(async () => {
    const [queue, requests, days, departments] = await Promise.all([
      api.getHrQueue(),
      api.getRequests({ scope: "all", from, to }),
      api.getCalendar(from, to),
      api.listDepartments(),
    ]);
    return { queue, requests, days, departments };
  }, [api, from, to]);
  const { data, error, loading, reload } = useLoad(load, [load]);
  const pending = (data?.queue.pendingApproval ?? []).filter((request) => filter === "all" || request.type === filter);
  const schedule = (data?.requests ?? []).filter((request) => {
    if (request.status === "rejected" || request.status === "cancelled" || request.status === "draft") return false;
    if (departmentId !== "all" && request.person.departmentId !== departmentId) return false;
    return true;
  });

  return (
    <div data-testid="hr-area">
      <p className="kicker">{t("hr.kicker")}</p>
      <h1 className="display">
        <span className="display-text">{t("hr.title")}</span>
      </h1>
      <p className="lead">
        {t("hr.myArea")} <Link to="/">{t("nav.employee")}</Link>
      </p>
      <div className="actions">
        <Link className="btn btn-yellow" to="/hr/reports">
          {t("hr.openReports")}
        </Link>
      </div>
      {loading ? <p className="muted">{t("common.loading")}</p> : null}
      {error ? <p className="banner banner-warn">{translateError(t, error)}</p> : null}

      <section>
        <div className="item-top">
          <h2 className="on-dark-title">{t("hr.queue")}</h2>
          <div className="seg" role="group">
            {filters.map((item) => (
              <button key={item} type="button" aria-pressed={filter === item} onClick={() => setFilter(item)}>
                {item === "all" ? t("hr.filterAll") : t(`request.type.${item}`)}
              </button>
            ))}
          </div>
        </div>
        {data && pending.length === 0 ? <p className="card empty">{t("hr.emptyQueue")}</p> : null}
        <div className="stack">
          {pending.map((request) => (
            <QueueCard
              key={request.id}
              request={request}
              onDecide={async (decision, comment) => {
                await api.decide(request.pendingStepId || request.id, {
                  decision,
                  comment: comment || undefined,
                });
                reload();
              }}
            />
          ))}
        </div>
      </section>

      <section className="stack">
        <h2 className="on-dark-title">{t("hr.processedQueue")}</h2>
        {data && data.queue.awaitingProcessed.length === 0 ? <p className="card empty">{t("hr.emptyProcessed")}</p> : null}
        {data?.queue.awaitingProcessed.map((request) => (
          <article key={request.id} className="card item-top">
            <div>
              <strong>{request.person.displayName}</strong>
              <div className="muted">
                {request.person.department} · {formatRange(request.start, request.end, language)}
              </div>
            </div>
            <button className="btn btn-yellow" type="button" onClick={() => void api.markProcessed(request.id).then(() => reload())}>
              {t("hr.markProcessed")}
            </button>
          </article>
        ))}
      </section>

      {data ? (
        <section className="plan">
          <div className="item-top">
            <h2 className="on-dark-title">{t("hr.schedule")}</h2>
            <label className="field inline">
              <span>{t("reports.department")}</span>
              <select value={departmentId} onChange={(event) => setDepartmentId(event.target.value)}>
                <option value="all">{t("hr.departmentAll")}</option>
                {data.departments.map((department) => (
                  <option key={department.id} value={department.id}>
                    {department.name}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <p className="muted">{t("hr.scheduleHint")}</p>
          <YearCalendar year={year} days={data.days} requests={schedule} />
        </section>
      ) : null}
    </div>
  );
}
