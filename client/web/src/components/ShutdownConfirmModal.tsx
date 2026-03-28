import BlockingModal from "./BlockingModal";

type ShutdownConfirmModalProps = {
  open: boolean;
  nodeUrl: string;
  loading: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

function ShutdownConfirmModal({ open, nodeUrl, loading, onCancel, onConfirm }: ShutdownConfirmModalProps) {
  return (
    <BlockingModal open={open}>
      <div>
        <div className="mb-4">
          <h2 className="text-lg font-semibold text-slate-900">Confirm Node Termination</h2>
          <p className="mt-2 text-sm leading-6 text-slate-600">
            Terminate the selected node at <span className="font-medium text-slate-900">{nodeUrl || "-"}</span>? This
            local demo action stops that node process so you can verify failover and leader re-election.
          </p>
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          <button
            type="button"
            onClick={onCancel}
            disabled={loading}
            className="rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={loading}
            className="rounded-md bg-red-700 px-4 py-2 text-sm font-medium text-white hover:bg-red-600 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {loading ? "Stopping..." : "Confirm Termination"}
          </button>
        </div>
      </div>
    </BlockingModal>
  );
}

export default ShutdownConfirmModal;
