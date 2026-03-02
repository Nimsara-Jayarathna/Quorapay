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
  );
}

export default NodeStatusPanel;
