import type { LeaveRequest } from "../api/types";

export type HrBlock = "self" | "certificate";

/** HR approves someone else's request. A sick day also needs a certificate number. */
export function hrBlock(request: LeaveRequest, meId: string): HrBlock | null {
  if (request.person.id === meId) return "self";
  if (request.type === "sick" && !request.certificateNumber) return "certificate";
  return null;
}
