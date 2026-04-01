import { NodeStatus } from "../lib/api";

type NodeStatusPanelProps = {
  status: NodeStatus | null;
  statusLoading: boolean;
  nodeActionLoading: boolean;
  shutdownMessage: string | null;
  selectedNodeId: string;
  onRefreshStatus: () => void;
  onNodeAction: (action: "stop" | "restart", nodeId: string) => void;
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
  status,
  statusLoading,
  nodeActionLoading,
  shutdownMessage,
  selectedNodeId,
  onRefreshStatus,
  onNodeAction,
}: NodeStatusPanelProps) {
  const operatingNodeId = selectedNodeId || "-";

  const infoRowsLeft = [
    { label: "Node ID", value: status?.node_id ?? "-" },
    { label: "Leader ID", value: status?.leader_id ?? "-" },
    { label: "Leader URL", value: status?.leader_url ?? "-" },
    { label: "Term", value: status?.term ?? "-" },
  ];

  const infoRowsRight = [
    { label: "Last Log Index", value: status?.last_log_index ?? "-" },
    { label: "Commit Index", value: status?.commit_index ?? "-" },
    { label: "Log Head", value: status?.log_head ?? "-" },
    { label: "Lamport Time", value: status?.lamport_time ?? "-" },
  ];

  const diagnosticsRows = [
    { label: "Fault Reason", value: status?.last_fault_reason ?? "-" },
    { label: "State Change", value: status?.last_state_change ?? "-" },
    { label: "ZooKeeper Error", value: status?.zk_error ?? "-" },
    { label: "Clock Skew (ms)", value: status?.clock_skew_ms ?? "-" },
    { label: "Status Refresh (ms)", value: status?.status_refresh_ms ?? "-" },
  ];

  return (
    <section className="h-full rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-medium text-slate-900">Node Details</h2>
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
            onClick={() => onNodeAction("stop", operatingNodeId)}
            disabled={nodeActionLoading || !selectedNodeId}
            className="rounded-md bg-red-700 px-3 py-2 text-sm font-medium text-white hover:bg-red-600 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {nodeActionLoading ? "Applying..." : `Kill ${operatingNodeId}`}
          </button>
          <button
            type="button"
            onClick={() => onNodeAction("restart", operatingNodeId)}
            disabled={nodeActionLoading || !selectedNodeId}
            className="rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {nodeActionLoading ? "Applying..." : `Restart ${operatingNodeId}`}
          </button>
        </div>
      </div>

      {shutdownMessage ? (
        <div className="mb-4 rounded-md border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          {shutdownMessage}
        </div>
      ) : null}

      <div className="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs font-medium uppercase tracking-wide text-slate-500">Node</p>
          <p className="mt-1 text-lg font-semibold text-slate-900">{status?.node_id ?? "-"}</p>
        </div>
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs font-medium uppercase tracking-wide text-slate-500">Role</p>
          <div className="mt-1">
            <span className={`inline-flex rounded-full border px-2 py-0.5 text-xs font-semibold ${getRoleClasses(status?.role)}`}>
              {status?.role ?? "-"}
            </span>
          </div>
        </div>
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs font-medium uppercase tracking-wide text-slate-500">Leader</p>
          <p className="mt-1 text-lg font-semibold text-slate-900">{status?.leader_id ?? "-"}</p>
        </div>
        <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs font-medium uppercase tracking-wide text-slate-500">Health</p>
          <div className="mt-1">
            <span className={`inline-flex rounded-full border px-2 py-0.5 text-xs font-semibold ${getFaultStateClasses(status?.fault_state)}`}>
              {status?.fault_state ?? "-"}
            </span>
          </div>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-md border border-slate-200">
          <div className="border-b border-slate-200 bg-slate-50 px-3 py-2 text-sm font-semibold text-slate-700">Cluster</div>
          <dl className="divide-y divide-slate-100">
            {infoRowsLeft.map((row) => (
              <div key={row.label} className="grid grid-cols-[140px_1fr] gap-3 px-3 py-2">
                <dt className="text-sm text-slate-500">{row.label}</dt>
                <dd className="break-all text-sm font-medium text-slate-900">{row.value}</dd>
              </div>
            ))}
          </dl>
        </div>

        <div className="rounded-md border border-slate-200">
          <div className="border-b border-slate-200 bg-slate-50 px-3 py-2 text-sm font-semibold text-slate-700">Replication</div>
          <dl className="divide-y divide-slate-100">
            {infoRowsRight.map((row) => (
              <div key={row.label} className="grid grid-cols-[140px_1fr] gap-3 px-3 py-2">
                <dt className="text-sm text-slate-500">{row.label}</dt>
                <dd className="text-sm font-medium text-slate-900">{row.value}</dd>
              </div>
            ))}
          </dl>
        </div>
      </div>

      <div className="mt-4 rounded-md border border-slate-200">
        <div className="border-b border-slate-200 bg-slate-50 px-3 py-2 text-sm font-semibold text-slate-700">Diagnostics</div>
        <dl className="divide-y divide-slate-100">
          {diagnosticsRows.map((row) => (
            <div key={row.label} className="grid grid-cols-[170px_1fr] gap-3 px-3 py-2">
              <dt className="text-sm text-slate-500">{row.label}</dt>
              <dd className="break-all text-sm font-medium text-slate-900">{row.value}</dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}

export default NodeStatusPanel;
