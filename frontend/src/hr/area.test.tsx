import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { createFixtureApi } from "../api/fixture";
import { I18nProvider } from "../i18n/context";
import { MeProvider, SessionProvider } from "../session";
import { HrArea } from "./HrArea";

describe("HR queue actions", () => {
  it("blocks the signed-in person's own request and a sick day without a certificate", async () => {
    const api = createFixtureApi({ today: "2026-11-13" });
    const me = await api.getMe();
    render(
      <I18nProvider initial="ru">
        <SessionProvider value={{ api, preview: null, logout: () => undefined }}>
          <MemoryRouter>
            <MeProvider me={me}>
              <HrArea />
            </MeProvider>
          </MemoryRouter>
        </SessionProvider>
      </I18nProvider>,
    );

    const articles = await screen.findAllByRole("article");
    const own = articles.find((article) => article.textContent?.includes("Соколова Анна") && article.textContent?.includes("Согласовать"));
    const uncovered = articles.find((article) => article.textContent?.includes("Морозова Мария") && article.textContent?.includes("Больничный"));
    const vacation = articles.find((article) => article.textContent?.includes("Морозова Мария") && article.textContent?.includes("Отпуск"));
    if (!own || !uncovered || !vacation) throw new Error("missing queue cards");

    const button = (root: HTMLElement, name: string) => within(root).getByRole("button", { name }) as HTMLButtonElement;
    expect(button(own, "Согласовать").disabled).toBe(true);
    expect(button(own, "Отклонить").disabled).toBe(true);
    expect(button(uncovered, "Согласовать").disabled).toBe(true);
    expect(button(uncovered, "Отклонить").disabled).toBe(false);
    expect(button(vacation, "Согласовать").disabled).toBe(false);
    expect(button(vacation, "Отклонить").disabled).toBe(false);
  });
});
