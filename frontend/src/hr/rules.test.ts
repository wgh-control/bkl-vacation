import { describe, expect, it } from "vitest";
import type { LeaveRequest } from "../api/types";
import { hrBlock } from "./rules";

function request(patch: Partial<LeaveRequest>): LeaveRequest {
  return {
    id: "req",
    type: "vacation",
    status: "pending_hr",
    person: { id: "emp-other", displayName: "Other" },
    start: "2027-03-01",
    end: "2027-03-15",
    countedDays: 14,
    createdAt: "2026-11-01T00:00:00",
    ...patch,
  };
}

describe("hr decision guard", () => {
  it("lets HR approve another person's vacation, day off, and certified sick day", () => {
    expect(hrBlock(request({ type: "vacation" }), "emp-anna")).toBeNull();
    expect(hrBlock(request({ type: "day_off" }), "emp-anna")).toBeNull();
    expect(hrBlock(request({ type: "sick", certificateNumber: "900" }), "emp-anna")).toBeNull();
  });

  it("stops an HR user approving their own request", () => {
    expect(hrBlock(request({ person: { id: "emp-anna", displayName: "Анна" } }), "emp-anna")).toBe("self");
  });

  it("stops approval of a sick day that has no certificate, and still names that case", () => {
    expect(hrBlock(request({ type: "sick" }), "emp-anna")).toBe("certificate");
  });
});
