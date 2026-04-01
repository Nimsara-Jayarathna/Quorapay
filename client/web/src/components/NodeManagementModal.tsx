import BlockingModal from "./BlockingModal";

type NodeAction = "start" | "stop" | "restart";

type NodeManagementModalProps = {
  open: boolean;
  action: NodeAction;
  targetNodeId: string;
  knownNodeIds: string[];
  adminToken: string;
  loading: boolean;
  message: string | null;
  onClose: () => void;
  onTargetNodeIdChange: (nodeId: string) => void;
  onAdminTokenChange: (token: string) => void;
  onConfirm: () => void;
};

function NodeManagementModal({
  open,
  action,
  targetNodeId,
  knownNodeIds,
  adminToken,
  loading,
  message,
  onClose,
  onTargetNodeIdChange,
  onAdminTokenChange,
  onConfirm,
}: NodeManagementModalProps) {
  const actionLabel = action === "start" ? "Start" : action === "stop" ? "Terminate" : "Restart";
  const allNodeIds = Array.from({ length: 26 }, (_, idx) => String.fromCharCode(65 + idx));
  const startableNodeIds = allNodeIds.filter((id) => !knownNodeIds.includes(id));
  const actionThemeClass =
    action === "start"
      ? "bg-emerald-700 hover:bg-emerald-600"
      : action === "stop"
        ? "bg-red-700 hover:bg-red-600"
        : "bg-amber-700 hover:bg-amber-600";

  return (
    <BlockingModal open={open}>
      <div className="space-y-4">
        <div>
          <h3 className="text-2xl font-semibold text-slate-900">Node Management</h3>
          <p className="mt-1 text-sm text-slate-600">{actionLabel} action selected. Choose target node and confirm.</p>
        </div>

        <div>
          <label htmlFor="node-management-target" className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">
            Target Node
          </label>
          <select
            id="node-management-target"
            value={targetNodeId}
            onChange={(event) => onTargetNodeIdChange(event.target.value)}
            className="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900"
          >
            {action === "start"
              ? allNodeIds.map((id) => {
                  const running = knownNodeIds.includes(id);
                  return (
                    <option key={id} value={id} disabled={running}>
                      Node {id}{running ? " (running)" : ""}
                    </option>
                  );
                })
              : knownNodeIds.map((id) => (
                  <option key={id} value={id}>
                    Node {id}
                  </option>
                ))}
          </select>
          {action === "start" ? (
            <p className="mt-1 text-xs text-slate-500">
              Running nodes are blocked. Available to start: {startableNodeIds.length > 0 ? startableNodeIds.join(", ") : "none"}.
            </p>
          ) : null}
        </div>

        <div>
          <label htmlFor="admin-token-modal" className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">
            Admin Token
          </label>
          <input
            id="admin-token-modal"
            type="password"
            value={adminToken}
            onChange={(event) => onAdminTokenChange(event.target.value)}
            placeholder="Bearer token for admin service"
            className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 outline-none ring-slate-200 placeholder:text-slate-400 focus:border-slate-500 focus:ring"
          />
        </div>

        {message ? <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-800">{message}</div> : null}

        <div className="flex items-center justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={loading}
            className={`rounded-md px-4 py-2 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-60 ${actionThemeClass}`}
          >
            {loading ? "Applying..." : `${actionLabel} Node`}
          </button>
        </div>
      </div>
    </BlockingModal>
  );
}

export default NodeManagementModal;
