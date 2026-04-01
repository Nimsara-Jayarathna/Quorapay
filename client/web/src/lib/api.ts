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
  lamport_time?: number;
  clock_skew_ms?: number;
};

export type PaymentRequest = {
  payment_id: string;
  amount: number;
  currency: string;
  simulate_outcome?: "SUCCESS" | "FAILED";
};

export type FollowerResult = {
  follower_base_url: string;
  append_acknowledged: boolean;
  commit_acknowledged: boolean;
  error?: string;
};

export type PaymentTrace = {
  received_by?: string;
  routed_to_leader: boolean;
  required_quorum?: number;
  ack_count?: number;
  follower_results?: FollowerResult[];
};

export type PaymentResponse = {
  payment_id: string;
  status: "OK" | string;
  message?: string;
  log_index?: number;
  term?: number;
  leader_id?: string;
  leader_url?: string;
  trace?: PaymentTrace;
};

export type LedgerItem = {
  log_index: number;
  logical_time?: number;
  payment_id: string;
  amount: number;
  currency: string;
  status: "COMMITTED" | "FAILED" | "PENDING" | "CANCELED" | string;
  created_at: string;
  received_by?: string;
  processed_by?: string;
  server_id?: string;
};

export type LedgerResponse = {
  count: number;
  items: LedgerItem[];
};

export type AdminNodeActionResponse = {
  status: "OK" | string;
  node_id: string;
  action: "start" | "stop" | "restart" | string;
  output?: string;
};

export type StatusFilter = "ALL" | "COMMITTED" | "FAILED" | "PENDING" | "CANCELED";

export type ClusterNode = {
  node_id: string;
  url: string;
};

export type ClusterNodesResponse = {
  count: number;
  items: ClusterNode[];
};

export type PaymentEvent = {
  timestamp: string;
  node_id: string;
  payment_id?: string;
  stage: string;
  message: string;
};

export type PaymentEventsResponse = {
  count: number;
  items: PaymentEvent[];
};

export function getErrorMessage(error: unknown): string {
  if (error instanceof TypeError) {
    const message = String(error.message || "").toLowerCase();
    if (message.includes("failed to fetch") || message.includes("networkerror")) {
      return "Cannot reach payment gateway. Ensure gateway is running on VITE_GATEWAY_API_BASE_URL and try again.";
    }
  }
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

export async function fetchClusterNodes(baseUrl: string): Promise<string[]> {
  const result = await fetchJson<ClusterNodesResponse>(`${baseUrl}/cluster/nodes`);
  const urls = (result.items ?? [])
    .map((item) => item.url?.trim())
    .filter((url): url is string => Boolean(url));
  return Array.from(new Set(urls));
}
