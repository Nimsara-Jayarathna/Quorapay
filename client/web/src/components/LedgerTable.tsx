import { LedgerItem, StatusFilter } from "../lib/api";

type LedgerTableProps = {
  statusFilter: StatusFilter;
  onStatusFilterChange: (value: StatusFilter) => void;
  ledgerLoading: boolean;
  ledgerError: string | null;
  items: LedgerItem[];
  onRefreshLedger: () => void;
};

function formatDate(isoDate: string): string {
  const date = new Date(isoDate);
  if (Number.isNaN(date.getTime())) {
    return isoDate;
  }
  return date.toLocaleString();
}

function LedgerTable({ statusFilter, onStatusFilterChange, ledgerLoading, ledgerError, items, onRefreshLedger }: LedgerTableProps) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-medium text-slate-900">Ledger</h2>
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={statusFilter}
            onChange={(event) => onStatusFilterChange(event.target.value as StatusFilter)}
            className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
          >
            <option value="ALL">All</option>
            <option value="COMMITTED">Committed</option>
            <option value="FAILED">Failed</option>
            <option value="PENDING">Pending</option>
          </select>
          <button
            type="button"
            onClick={onRefreshLedger}
            disabled={ledgerLoading}
            aria-label="Refresh ledger"
            title="Refresh ledger"
            className="inline-flex h-10 w-10 items-center justify-center rounded-md bg-slate-900 text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            <svg viewBox="0 0 24 24" className={`h-5 w-5 ${ledgerLoading ? "animate-spin" : ""}`} fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path d="M20 12a8 8 0 0 1-13.66 5.66" />
              <path d="M4 12a8 8 0 0 1 13.66-5.66" />
              <path d="M7 17H4v3" />
              <path d="M17 7h3V4" />
            </svg>
          </button>
        </div>
      </div>

      {ledgerError ? <div className="mb-3 text-sm text-red-700">Ledger error: {ledgerError}</div> : null}

      <div className="overflow-x-auto rounded-md border border-slate-200">
        <table className="min-w-full divide-y divide-slate-200 text-sm">
          <thead className="bg-slate-50">
            <tr>
              <th className="px-3 py-2 text-left font-medium text-slate-600">Log Index</th>
              <th className="px-3 py-2 text-left font-medium text-slate-600">Logical Time</th>
              <th className="px-3 py-2 text-left font-medium text-slate-600">Payment ID</th>
              <th className="px-3 py-2 text-left font-medium text-slate-600">Amount</th>
              <th className="px-3 py-2 text-left font-medium text-slate-600">Currency</th>
              <th className="px-3 py-2 text-left font-medium text-slate-600">Status</th>
              <th className="px-3 py-2 text-left font-medium text-slate-600">Received By</th>
              <th className="px-3 py-2 text-left font-medium text-slate-600">Processed By</th>
              <th className="px-3 py-2 text-left font-medium text-slate-600">Created At</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 bg-white">
            {items.length === 0 ? (
              <tr>
                <td className="px-3 py-4 text-slate-500" colSpan={9}>
                  No ledger entries to display.
                </td>
              </tr>
            ) : (
              items.map((item) => (
                <tr key={`${item.log_index}-${item.payment_id}`}>
                  <td className="px-3 py-2 text-slate-700">{item.log_index}</td>
                  <td className="px-3 py-2 text-slate-700">{item.logical_time ?? "-"}</td>
                  <td className="px-3 py-2 font-mono text-xs text-slate-700">{item.payment_id}</td>
                  <td className="px-3 py-2 text-slate-700">{item.amount}</td>
                  <td className="px-3 py-2 text-slate-700">{item.currency}</td>
                  <td className="px-3 py-2 text-slate-700">{item.status}</td>
                  <td className="px-3 py-2 text-slate-700">{item.received_by ?? "-"}</td>
                  <td className="px-3 py-2 text-slate-700">{item.processed_by ?? "-"}</td>
                  <td className="px-3 py-2 text-slate-700">{formatDate(item.created_at)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}

export default LedgerTable;
