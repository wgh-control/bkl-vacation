import { useCallback, useState } from "react";
import { Link } from "react-router-dom";
import { ApiError } from "../api/errors";
import type { ExportScope } from "../api/types";
import { calendarYear, todayISO, yearBounds } from "../domain/dates";
import { translateError, useI18n } from "../i18n/context";
import { useApi } from "../session";
import { useLoad } from "../useLoad";

function saveFile(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function Reports() {
  const api = useApi();
  const { t } = useI18n();
  const year = calendarYear(todayISO());
  const bounds = yearBounds(year);
  const [scope, setScope] = useState<ExportScope>("all");
  const [personId, setPersonId] = useState("");
  const [query, setQuery] = useState("");
  const [departmentId, setDepartmentId] = useState("");
  const [from, setFrom] = useState(bounds.from);
  const [to, setTo] = useState(bounds.to);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const load = useCallback(async () => {
    const [people, departments] = await Promise.all([api.listPeople(query), api.listDepartments()]);
    return { people, departments };
  }, [api, query]);
  const { data, error: loadError } = useLoad(load, [load]);

  const needsPerson = scope === "person" && !personId;
  const needsDepartment = scope === "department" && !departmentId;
  const blocked = needsPerson || needsDepartment || pending;

  async function download(format: "pdf" | "xls") {
    setPending(true);
    setError(null);
    setMessage(null);
    try {
      const file = await api.exportReport({
        format,
        scope,
        personId: scope === "person" ? personId : undefined,
        departmentId: scope === "department" ? departmentId : undefined,
        from,
        to,
      });
      saveFile(file.blob, file.filename);
      setMessage(file.filename);
    } catch (reason) {
      setError(reason instanceof ApiError ? reason.code : "unknown");
    } finally {
      setPending(false);
    }
  }

  return (
    <div data-testid="hr-reports">
      <Link className="back" to="/hr">
        {t("common.back")}
      </Link>
      <p className="kicker">{t("reports.kicker")}</p>
      <h1 className="display">
        <span className="display-text">{t("reports.title")}</span>
      </h1>
      <p className="lead">{t("reports.lead")}</p>
      {loadError ? <p className="banner banner-warn">{translateError(t, loadError)}</p> : null}
      <form
        className="card stack"
        onSubmit={(event) => {
          event.preventDefault();
        }}
      >
        <fieldset className="field">
          <legend>{t("reports.scope")}</legend>
          <div className="seg">
            {(["person", "department", "all"] as ExportScope[]).map((item) => (
              <label key={item}>
                <input
                  type="radio"
                  name="scope"
                  checked={scope === item}
                  onChange={() => setScope(item)}
                  data-testid={`report-scope-${item}`}
                />
                {t(`reports.scope${item === "person" ? "Person" : item === "department" ? "Department" : "All"}`)}
              </label>
            ))}
          </div>
        </fieldset>
        {scope === "person" ? (
          <div className="field">
            <label>
              <span>{t("reports.personSearch")}</span>
              <input value={query} onChange={(event) => setQuery(event.target.value)} />
            </label>
            <div className="choices">
              {data?.people.map((person) => (
                <label key={person.id}>
                  <input type="radio" name="person" checked={personId === person.id} onChange={() => setPersonId(person.id)} />
                  {person.displayName} · {person.employeeNumber} · {person.department}
                </label>
              ))}
            </div>
            {needsPerson ? <span className="muted">{t("reports.needPerson")}</span> : null}
          </div>
        ) : null}
        {scope === "department" ? (
          <label className="field">
            <span>{t("reports.department")}</span>
            <select value={departmentId} onChange={(event) => setDepartmentId(event.target.value)}>
              <option value="">{t("reports.needDepartment")}</option>
              {data?.departments.map((department) => (
                <option key={department.id} value={department.id}>
                  {department.name}
                </option>
              ))}
            </select>
          </label>
        ) : null}
        <div className="fields-2">
          <label className="field">
            <span>{t("common.from")}</span>
            <input type="date" value={from} onChange={(event) => setFrom(event.target.value)} />
          </label>
          <label className="field">
            <span>{t("common.to")}</span>
            <input type="date" value={to} onChange={(event) => setTo(event.target.value)} />
          </label>
        </div>
        {error ? <p className="banner banner-warn">{translateError(t, error)}</p> : null}
        {message ? (
          <p className="banner banner-info">
            {t("reports.ready")}: {message}
          </p>
        ) : null}
        <div className="actions">
          <button className="btn btn-primary" type="button" disabled={blocked} data-testid="download-pdf" onClick={() => void download("pdf")}>
            {t("reports.pdf")}
          </button>
          <button className="btn btn-yellow" type="button" disabled={blocked} data-testid="download-xls" onClick={() => void download("xls")}>
            {t("reports.xls")}
          </button>
        </div>
      </form>
    </div>
  );
}
