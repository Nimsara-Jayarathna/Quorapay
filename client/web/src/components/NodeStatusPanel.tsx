import { NodeStatus } from "../lib/api";

type NodeStatusPanelProps = {
  nodeUrls: string[];
  selectedNodeIndex: number;
  onSelectNode: (index: number) => void;
  status: NodeStatus | null;
  statusLoading: boolean;
  shutdownLoading: boolean;
  shutdownMessage: string | null;
  onRefreshStatus: () => void;
  onRequestTerminate: () => void;
};

function getFaultStateClasses(state: string | undefined): string {
  switch (state) {
    case "HEALTHY":
      return "bg-emerald-100 text-emerald-800 border-emerald-200";
    case "FAILED":
      return "bg-red-100 text-red-800 border-red-200";
    case "RECOVERING":
      return "bg-amber-100 text-amber-800 border-amber-200";
    case "REJOINED":
      return "bg-blue-100 text-blue-800 border-blue-200";
    default:
      return "bg-slate-100 text-slate-700 border-slate-200";
  }
}

function getRoleClasses(role: string | undefined): string {
  switch (role) {
    case "LEADER":
      return "bg-indigo-100 text-indigo-800 border-indigo-200";
    case "FOLLOWER":
      return "bg-slate-100 text-slate-800 border-slate-200";
    default:
      return "bg-zinc-100 text-zinc-700 border-zinc-200";
  }
}

function NodeStatusPanel({
  nodeUrls,
  selectedNodeIndex,
  onSelectNode,
  status,
  statusLoading,
  shutdownLoading,
  shutdownMessage,
  onRefreshStatus,
  onRequestTerminate,
}: NodeStatusPanelProps) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-medium text-slate-900">Node Status</h2>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={onRefreshStatus}
            disabled={statusLoading}
            aria-label="Refresh status"
            title="Refresh status"
            className="inline-flex h-10 w-10 items-center justify-center rounded-md bg-slate-900 text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            <svg viewBox="0 0 24 24" className={`h-5 w-5 ${statusLoading ? "animate-spin" : ""}`} fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path d="M20 12a8 8 0 0 1-13.66 5.66" />
              <path d="M4 12a8 8 0 0 1 13.66-5.66" />
              <path d="M7 17H4v3" />
              <path d="M17 7h3V4" />
            </svg>
          </button>
          <button
            type="button"
            onClick={onRequestTerminate}
            disabled={shutdownLoading || nodeUrls.length === 0}
            className="rounded-md bg-red-700 px-3 py-2 text-sm font-medium text-white hover:bg-red-600 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {shutdownLoading ? "Stopping..." : "Terminate Selected Node"}
          </button>
        </div>
      </div>

      {shutdownMessage ? (
        <div className="mb-4 rounded-md border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          {shutdownMessage}
        </div>
      ) : null}

      <div className="mb-4 grid gap-3 sm:grid-cols-[220px_1fr] sm:items-center">
        <label htmlFor="node-select" className="text-sm font-medium text-slate-700">
          Active Node
        </label>
        <select
          id="node-select"
          value={selectedNodeIndex}
          onChange={(event) => onSelectNode(Number(event.target.value))}
          className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
        >
          {nodeUrls.map((url, index) => (
            <option key={url} value={index}>
              {`Node ${index + 1} - ${url}`}
            </option>
          ))}
        </select>
      </div>

      <div className="overflow-hidden rounded-md border border-slate-200">
        <dl className="grid grid-cols-1 divide-y divide-slate-200 text-sm sm:grid-cols-2 sm:divide-y-0 sm:divide-x">
          <div className="p-3">
            <dt className="font-medium text-slate-500">Node ID</dt>
            <dd className="mt-1 text-slate-900">{status?.node_id ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Role</dt>
            <dd className="mt-1">
              <span className={`inline-flex rounded-full border px-2 py-0.5 text-xs font-semibold ${getRoleClasses(status?.role)}`}>
                {status?.role ?? "-"}
              </span>
            </dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Leader ID</dt>
            <dd className="mt-1 text-slate-900">{status?.leader_id ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Leader URL</dt>
            <dd className="mt-1 break-all text-slate-900">{status?.leader_url ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Last Log Index</dt>
            <dd className="mt-1 text-slate-900">{status?.last_log_index ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Commit Index</dt>
            <dd className="mt-1 text-slate-900">{status?.commit_index ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Fault State</dt>
            <dd className="mt-1">
              <span
                className={`inline-flex rounded-full border px-2 py-0.5 text-xs font-semibold ${getFaultStateClasses(
                  status?.fault_state,
                )}`}
              >
                {status?.fault_state ?? "-"}
              </span>
            </dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Last Fault Reason</dt>
            <dd className="mt-1 text-slate-900">{status?.last_fault_reason ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Last State Change</dt>
            <dd className="mt-1 text-slate-900">{status?.last_state_change ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">ZooKeeper Error</dt>
            <dd className="mt-1 break-all text-slate-900">{status?.zk_error ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Term</dt>
            <dd className="mt-1 text-slate-900">{status?.term ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Log Head</dt>
            <dd className="mt-1 text-slate-900">{status?.log_head ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Status Refresh (ms)</dt>
            <dd className="mt-1 text-slate-900">{status?.status_refresh_ms ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Lamport Time</dt>
            <dd className="mt-1 text-slate-900">{status?.lamport_time ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">Clock Skew (ms)</dt>
            <dd className="mt-1 text-slate-900">{status?.clock_skew_ms ?? "-"}</dd>
          </div>
        </dl>
      </div>
    </section>
  );
}

export default NodeStatusPanel;
