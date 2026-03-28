export type NodeStatus = {
  node_id: string;
  role: "LEADER" | "FOLLOWER" | string;
  leader_id?: string;
  leader_url?: string;
  last_log_index?: number;
  commit_index?: number;
  fault_state?: "HEALTHY" | "FAILED" | "RECOVERING" | "REJOINED" | string;
  last_fault_reason?: string;
  last_state_change?: string;
  zk_error?: string;
  term?: number;
  log_head?: number;
  status_refresh_ms?: number;
};

export type PaymentRequest = {
  payment_id: string;
  amount: number;
  currency: string;
  note?: string;
};

export type PaymentResponse = {
  payment_id: string;
  accepted: boolean;
  status: "PENDING" | "COMMITTED" | "FAILED" | string;
  message: string;
  leader_url?: string;
  record?: Record<string, unknown>;
};

export type LedgerItem = {
  log_index: number;
  logical_time?: number;
  payment_id: string;
  amount: number;
  currency: string;
  status: "COMMITTED" | "FAILED" | "PENDING" | string;
  created_at: string;
  received_by?: string;
  processed_by?: string;
  server_id?: string;
};

export type LedgerResponse = {
  count: number;
  items: LedgerItem[];
};

export type ShutdownResponse = {
  message: string;
  node_id?: string;
};

export type StatusFilter = "ALL" | "COMMITTED" | "FAILED" | "PENDING";

export function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "Unexpected error";
}

export async function fetchJson<T>(url: string, init?: RequestInit): Promise<T> {
  const controller = new AbortController();
  const timeoutId = window.setTimeout(() => controller.abort(), 8000);

  const headers = new Headers(init?.headers);
  if (init?.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  try {
    const response = await fetch(url, {
      ...init,
      headers,
      signal: controller.signal,
    });

    const raw = await response.text();
    let payload: unknown = null;
    if (raw) {
      try {
        payload = JSON.parse(raw) as unknown;
      } catch {
        payload = raw;
      }
    }

    if (!response.ok) {
      if (payload && typeof payload === "object" && "message" in payload) {
        throw new Error(String((payload as { message?: unknown }).message ?? `HTTP ${response.status}`));
      }
      throw new Error(`HTTP ${response.status}`);
    }

    return payload as T;
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new Error("Request timed out");
    }
    throw error;
  } finally {
    window.clearTimeout(timeoutId);
  }
}
