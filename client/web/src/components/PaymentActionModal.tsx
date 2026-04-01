import BlockingModal from "./BlockingModal";

type PaymentActionModalProps = {
  open: boolean;
  state: "loading" | "success" | "error" | "info";
  title: string;
  message: string;
  canClose: boolean;
  onClose: () => void;
};

function PaymentActionModal({ open, state, title, message, canClose, onClose }: PaymentActionModalProps) {
  const isLoading = state === "loading";
  const isSuccess = state === "success";
  const isError = state === "error";
  const isInfo = state === "info";
  const toneClass = isSuccess
    ? "border-emerald-200 bg-emerald-50/90 text-emerald-800"
    : isError
      ? "border-red-200 bg-red-50/90 text-red-800"
      : isInfo
        ? "border-amber-200 bg-amber-50/90 text-amber-800"
      : "border-slate-200 bg-slate-100/90 text-slate-700";
  const iconWrapperClass = isSuccess
    ? "bg-emerald-100 text-emerald-700 ring-emerald-200"
    : isError
      ? "bg-red-100 text-red-700 ring-red-200"
      : isInfo
        ? "bg-amber-100 text-amber-700 ring-amber-200"
      : "bg-slate-100 text-slate-700 ring-slate-200";
  const statusIcon = isSuccess ? "✓" : isError ? "!" : isInfo ? "i" : "…";

  return (
    <BlockingModal open={open}>
      <div className="space-y-4 text-center">
        <div className="flex flex-col items-center gap-3">
          <span className={`inline-flex h-9 w-9 items-center justify-center rounded-full text-base font-semibold ring-1 ${iconWrapperClass}`}>
            {statusIcon}
          </span>
          <div className="min-w-0">
            <h2 className="text-lg font-semibold leading-6 text-slate-900">{title}</h2>
            <p className="mt-1 text-sm text-slate-600">Payment request workflow status</p>
          </div>
        </div>

        <p className={`whitespace-pre-line rounded-lg border px-3 py-3 text-sm leading-6 ${toneClass}`}>{message}</p>

        {isLoading ? (
          <div className="flex items-center justify-center gap-2 text-sm text-slate-600">
            <span className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-slate-400 border-t-transparent" />
            <span>Submitting to leader and waiting for cluster quorum...</span>
          </div>
        ) : null}

        {canClose ? (
          <div className="flex justify-center">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              Close
            </button>
          </div>
        ) : null}
      </div>
    </BlockingModal>
  );
}

export default PaymentActionModal;
