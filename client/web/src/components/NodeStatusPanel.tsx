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
        <h2 className="text-lg font-medium text-slate-900">Node Selector + Status</h2>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={onRefreshStatus}
            disabled={statusLoading}
            className="rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {statusLoading ? "Refreshing..." : "Refresh Status"}
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
          Active node
        </label>
        <select
          id="node-select"
          value={selectedNodeIndex}
          onChange={(event) => onSelectNode(Number(event.target.value))}
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
            <dd className="mt-1">
              <span className={`inline-flex rounded-full border px-2 py-0.5 text-xs font-semibold ${getRoleClasses(status?.role)}`}>
                {status?.role ?? "-"}
              </span>
            </dd>
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
          <div className="p-3">
            <dt className="font-medium text-slate-500">fault_state</dt>
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
            <dt className="font-medium text-slate-500">last_fault_reason</dt>
            <dd className="mt-1 text-slate-900">{status?.last_fault_reason ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">last_state_change</dt>
            <dd className="mt-1 text-slate-900">{status?.last_state_change ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">zk_error</dt>
            <dd className="mt-1 break-all text-slate-900">{status?.zk_error ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">term</dt>
            <dd className="mt-1 text-slate-900">{status?.term ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">log_head</dt>
            <dd className="mt-1 text-slate-900">{status?.log_head ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">status_refresh_ms</dt>
            <dd className="mt-1 text-slate-900">{status?.status_refresh_ms ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">lamport_time</dt>
            <dd className="mt-1 text-slate-900">{status?.lamport_time ?? "-"}</dd>
          </div>
          <div className="p-3">
            <dt className="font-medium text-slate-500">clock_skew_ms</dt>
            <dd className="mt-1 text-slate-900">{status?.clock_skew_ms ?? "-"}</dd>
          </div>
        </dl>
      </div>
    </section>
  );
}

export default NodeStatusPanel;
