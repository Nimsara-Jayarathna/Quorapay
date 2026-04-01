import BlockingModal from "./BlockingModal";

type NoNodesConnectedModalProps = {
  open: boolean;
  onRetry: () => void;
};

function NoNodesConnectedModal({ open, onRetry }: NoNodesConnectedModalProps) {
  return (
    <BlockingModal open={open}>
      <div className="space-y-4 text-center">
        <h2 className="text-lg font-semibold text-slate-900">No Nodes Connected</h2>
        <p className="text-sm leading-6 text-slate-600">
          The web client cannot reach any Quorapay node right now. Start the node cluster and retry connectivity.
        </p>
        <div className="flex justify-center">
          <button
            type="button"
            onClick={onRetry}
            className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800"
          >
            Retry Connection
          </button>
        </div>
      </div>
    </BlockingModal>
  );
}

export default NoNodesConnectedModal;
