import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";

type NodeStatus = {
  node_id: string;
  role: "LEADER" | "FOLLOWER" | string;
  leader_id?: string;
  leader_url?: string;
  last_log_index?: number;
  commit_index?: number;
};

type PaymentRequest = {
  payment_id: string;
  amount: number;
  currency: string;
  note?: string;
};

type PaymentResponse = {
  payment_id: string;
  accepted: boolean;
  status: "PENDING" | "COMMITTED" | "FAILED" | string;
  message: string;
  leader_url?: string;
  record?: Record<string, unknown>;
};

type LedgerItem = {
  log_index: number;
  payment_id: string;
  amount: number;
  currency: string;
  status: "COMMITTED" | "FAILED" | "PENDING" | string;
  created_at: string;
  server_id?: string;
};

type LedgerResponse = {
  count: number;
  items: LedgerItem[];
};

type StatusFilter = "ALL" | "COMMITTED" | "FAILED" | "PENDING";

const nodeUrls = (import.meta.env.VITE_NODE_URLS as string | undefined)
  ?.split(",")
  .map((url) => url.trim())
  .filter(Boolean) ?? [];

const configuredDefaultIndex = Number(import.meta.env.VITE_DEFAULT_NODE_INDEX ?? "0");
const defaultNodeIndex =
  Number.isInteger(configuredDefaultIndex) && configuredDefaultIndex >= 0 && configuredDefaultIndex < nodeUrls.length
    ? configuredDefaultIndex
    : 0;

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "Unexpected error";
}

async function fetchJson<T>(url: string, init?: RequestInit): Promise<T> {
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

function formatDate(isoDate: string): string {
  const date = new Date(isoDate);
  if (Number.isNaN(date.getTime())) {
    return isoDate;
  }
  return date.toLocaleString();
}

function generatePaymentId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `pay-${Date.now()}-${Math.random().toString(16).slice(2, 10)}`;
}

function App() {
  const [selectedNodeIndex, setSelectedNodeIndex] = useState(defaultNodeIndex);

  const selectedNodeUrl = nodeUrls[selectedNodeIndex] ?? "";

  const [status, setStatus] = useState<NodeStatus | null>(null);
  const [statusLoading, setStatusLoading] = useState(false);
  const [statusError, setStatusError] = useState<string | null>(null);

  const [paymentId, setPaymentId] = useState(generatePaymentId());
  const [amount, setAmount] = useState("10.00");
  const [currency, setCurrency] = useState("USD");
  const [note, setNote] = useState("");
  const [paymentLoading, setPaymentLoading] = useState(false);
  const [paymentError, setPaymentError] = useState<string | null>(null);
  const [paymentResult, setPaymentResult] = useState<PaymentResponse | null>(null);

  const [ledgerItems, setLedgerItems] = useState<LedgerItem[]>([]);
  const [ledgerLoading, setLedgerLoading] = useState(false);
  const [ledgerError, setLedgerError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("ALL");

  const filteredLedgerItems = useMemo(() => {
    if (statusFilter === "ALL") {
      return ledgerItems;
    }
    return ledgerItems.filter((item) => item.status === statusFilter);
  }, [ledgerItems, statusFilter]);

  const refreshStatus = useCallback(async () => {
    if (!selectedNodeUrl) {
      setStatus(null);
      setStatusError("Node unreachable: no node URL configured. Check VITE_NODE_URLS.");
      return;
    }

    setStatusLoading(true);
    try {
      const result = await fetchJson<NodeStatus>(`${selectedNodeUrl}/status`);
      setStatus(result);
      setStatusError(null);
    } catch (error) {
      setStatus(null);
      setStatusError(`Node unreachable: ${getErrorMessage(error)}`);
    } finally {
      setStatusLoading(false);
    }
  }, [selectedNodeUrl]);

  const refreshLedger = useCallback(async () => {
    if (!selectedNodeUrl) {
      setLedgerItems([]);
      setLedgerError("No node URL configured. Check VITE_NODE_URLS.");
      return;
    }

    setLedgerLoading(true);
    try {
      const result = await fetchJson<LedgerResponse>(`${selectedNodeUrl}/ledger`);
      setLedgerItems(Array.isArray(result.items) ? result.items : []);
      setLedgerError(null);
    } catch (error) {
      setLedgerItems([]);
      setLedgerError(getErrorMessage(error));
    } finally {
      setLedgerLoading(false);
    }
  }, [selectedNodeUrl]);

  async function handleSubmitPayment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!selectedNodeUrl) {
      setPaymentError("No node URL configured. Check VITE_NODE_URLS.");
      return;
    }

    const numericAmount = Number(amount);
    if (!paymentId.trim()) {
      setPaymentError("payment_id is required.");
      return;
    }
    if (!Number.isFinite(numericAmount) || numericAmount <= 0) {
      setPaymentError("amount must be a positive number.");
      return;
    }
    if (!currency.trim()) {
      setPaymentError("currency is required.");
      return;
    }

    const payload: PaymentRequest = {
      payment_id: paymentId.trim(),
      amount: numericAmount,
      currency: currency.trim().toUpperCase(),
      note: note.trim() || undefined,
    };

    setPaymentLoading(true);
    setPaymentError(null);

    try {
      const result = await fetchJson<PaymentResponse>(`${selectedNodeUrl}/pay`, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      setPaymentResult(result);
      void refreshStatus();
      void refreshLedger();
    } catch (error) {
      setPaymentResult(null);
      setPaymentError(getErrorMessage(error));
    } finally {
      setPaymentLoading(false);
    }
  }

  useEffect(() => {
    void refreshStatus();
    void refreshLedger();
  }, [refreshStatus, refreshLedger]);

  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold text-slate-900">Quorapay Web Client</h1>
        <p className="mt-1 text-sm text-slate-600">Submit payments, inspect node status, and view replicated ledger data.</p>
      </header>

      {nodeUrls.length === 0 ? (
        <div className="mb-6 rounded-md border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          No nodes configured. Add `VITE_NODE_URLS` in `.env` based on `.env.example`.
        </div>
      ) : null}

      {statusError ? (
        <div className="mb-6 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">{statusError}</div>
      ) : null}

      <div className="space-y-6">
        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-lg font-medium text-slate-900">Node Selector + Status</h2>
            <button
              type="button"
              onClick={() => void refreshStatus()}
              disabled={statusLoading}
              className="rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {statusLoading ? "Refreshing..." : "Refresh Status"}
            </button>
          </div>

          <div className="mb-4 grid gap-3 sm:grid-cols-[220px_1fr] sm:items-center">
            <label htmlFor="node-select" className="text-sm font-medium text-slate-700">
              Active node
            </label>
            <select
              id="node-select"
              value={selectedNodeIndex}
              onChange={(event) => setSelectedNodeIndex(Number(event.target.value))}
              className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
            >
              {nodeUrls.map((url, index) => (
                <option key={url} value={index}>
                  {`Node ${String.fromCharCode(65 + index)} - ${url}`}
                </option>
              ))}
            </select>
          </div>

          <div className="overflow-hidden rounded-md border border-slate-200">
            <dl className="grid grid-cols-1 divide-y divide-slate-200 text-sm sm:grid-cols-2 sm:divide-y-0 sm:divide-x">
              <div className="p-3">
                <dt className="font-medium text-slate-500">node_id</dt>
                <dd className="mt-1 text-slate-900">{status?.node_id ?? "-"}</dd>
              </div>
              <div className="p-3">
                <dt className="font-medium text-slate-500">role</dt>
                <dd className="mt-1 text-slate-900">{status?.role ?? "-"}</dd>
              </div>
              <div className="p-3">
                <dt className="font-medium text-slate-500">leader_id</dt>
                <dd className="mt-1 text-slate-900">{status?.leader_id ?? "-"}</dd>
              </div>
              <div className="p-3">
                <dt className="font-medium text-slate-500">leader_url</dt>
                <dd className="mt-1 break-all text-slate-900">{status?.leader_url ?? "-"}</dd>
              </div>
              <div className="p-3">
                <dt className="font-medium text-slate-500">last_log_index</dt>
                <dd className="mt-1 text-slate-900">{status?.last_log_index ?? "-"}</dd>
              </div>
              <div className="p-3">
                <dt className="font-medium text-slate-500">commit_index</dt>
                <dd className="mt-1 text-slate-900">{status?.commit_index ?? "-"}</dd>
              </div>
            </dl>
          </div>
        </section>

        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <h2 className="mb-4 text-lg font-medium text-slate-900">Create Payment</h2>
          <form className="grid gap-4" onSubmit={handleSubmitPayment}>
            <div className="grid gap-2">
              <label htmlFor="payment-id" className="text-sm font-medium text-slate-700">
                payment_id
              </label>
              <div className="flex flex-col gap-2 sm:flex-row">
                <input
                  id="payment-id"
                  value={paymentId}
                  onChange={(event) => setPaymentId(event.target.value)}
                  className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
                  placeholder="Unique idempotency key"
                />
                <button
                  type="button"
                  onClick={() => setPaymentId(generatePaymentId())}
                  className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
                >
                  Generate payment_id
                </button>
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="grid gap-2">
                <label htmlFor="amount" className="text-sm font-medium text-slate-700">
                  amount
                </label>
                <input
                  id="amount"
                  type="number"
                  min="0"
                  step="0.01"
                  value={amount}
                  onChange={(event) => setAmount(event.target.value)}
                  className="rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
                />
              </div>
              <div className="grid gap-2">
                <label htmlFor="currency" className="text-sm font-medium text-slate-700">
                  currency
                </label>
                <input
                  id="currency"
                  value={currency}
                  onChange={(event) => setCurrency(event.target.value)}
                  className="rounded-md border border-slate-300 px-3 py-2 text-sm uppercase focus:border-slate-400 focus:outline-none"
                  placeholder="USD"
                />
              </div>
            </div>

            <div className="grid gap-2">
              <label htmlFor="note" className="text-sm font-medium text-slate-700">
                note (optional)
              </label>
              <input
                id="note"
                value={note}
                onChange={(event) => setNote(event.target.value)}
                className="rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
                placeholder="Optional note"
              />
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <button
                type="submit"
                disabled={paymentLoading}
                className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {paymentLoading ? "Submitting..." : "Submit Payment"}
              </button>
              {paymentError ? <span className="text-sm text-red-700">{paymentError}</span> : null}
              {paymentResult ? <span className="text-sm text-slate-600">{`${paymentResult.status}: ${paymentResult.message}`}</span> : null}
            </div>
          </form>

          <div className="mt-4">
            <h3 className="text-sm font-medium text-slate-700">Response</h3>
            <pre className="mt-2 max-h-72 overflow-auto rounded-md bg-slate-900 p-3 font-mono text-xs text-slate-100">
              {paymentResult ? JSON.stringify(paymentResult, null, 2) : "No response yet."}
            </pre>
          </div>
        </section>

        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-lg font-medium text-slate-900">Ledger Viewer</h2>
            <div className="flex flex-wrap items-center gap-2">
              <select
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value as StatusFilter)}
                className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
              >
                <option value="ALL">All</option>
                <option value="COMMITTED">Committed</option>
                <option value="FAILED">Failed</option>
                <option value="PENDING">Pending</option>
              </select>
              <button
                type="button"
                onClick={() => void refreshLedger()}
                disabled={ledgerLoading}
                className="rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {ledgerLoading ? "Refreshing..." : "Refresh Ledger"}
              </button>
            </div>
          </div>

          {ledgerError ? <div className="mb-3 text-sm text-red-700">Ledger error: {ledgerError}</div> : null}

          <div className="overflow-x-auto rounded-md border border-slate-200">
            <table className="min-w-full divide-y divide-slate-200 text-sm">
              <thead className="bg-slate-50">
                <tr>
                  <th className="px-3 py-2 text-left font-medium text-slate-600">log_index</th>
                  <th className="px-3 py-2 text-left font-medium text-slate-600">payment_id</th>
                  <th className="px-3 py-2 text-left font-medium text-slate-600">amount</th>
                  <th className="px-3 py-2 text-left font-medium text-slate-600">currency</th>
                  <th className="px-3 py-2 text-left font-medium text-slate-600">status</th>
                  <th className="px-3 py-2 text-left font-medium text-slate-600">created_at</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 bg-white">
                {filteredLedgerItems.length === 0 ? (
                  <tr>
                    <td className="px-3 py-4 text-slate-500" colSpan={6}>
                      No ledger entries to display.
                    </td>
                  </tr>
                ) : (
                  filteredLedgerItems.map((item) => (
                    <tr key={`${item.log_index}-${item.payment_id}`}>
                      <td className="px-3 py-2 text-slate-700">{item.log_index}</td>
                      <td className="px-3 py-2 font-mono text-xs text-slate-700">{item.payment_id}</td>
                      <td className="px-3 py-2 text-slate-700">{item.amount}</td>
                      <td className="px-3 py-2 text-slate-700">{item.currency}</td>
                      <td className="px-3 py-2 text-slate-700">{item.status}</td>
                      <td className="px-3 py-2 text-slate-700">{formatDate(item.created_at)}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </div>
  );
}

export default App;
