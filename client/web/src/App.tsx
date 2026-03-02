import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";

import LedgerTable from "./components/LedgerTable";
import NodeStatusPanel from "./components/NodeStatusPanel";
import PaymentForm from "./components/PaymentForm";
import ShutdownConfirmModal from "./components/ShutdownConfirmModal";
import {
  fetchJson,
  getErrorMessage,
  LedgerResponse,
  NodeStatus,
  PaymentRequest,
  PaymentResponse,
  ShutdownResponse,
  StatusFilter,
} from "./lib/api";

const nodeUrls = (import.meta.env.VITE_NODE_URLS as string | undefined)
  ?.split(",")
  .map((url) => url.trim())
  .filter(Boolean) ?? [];

const configuredDefaultIndex = Number(import.meta.env.VITE_DEFAULT_NODE_INDEX ?? "0");
const defaultNodeIndex =
  Number.isInteger(configuredDefaultIndex) && configuredDefaultIndex >= 0 && configuredDefaultIndex < nodeUrls.length
    ? configuredDefaultIndex
    : 0;

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

  const [ledgerItems, setLedgerItems] = useState<LedgerResponse["items"]>([]);
  const [ledgerLoading, setLedgerLoading] = useState(false);
  const [ledgerError, setLedgerError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("ALL");

  const [shutdownLoading, setShutdownLoading] = useState(false);
  const [shutdownMessage, setShutdownMessage] = useState<string | null>(null);
  const [showShutdownConfirm, setShowShutdownConfirm] = useState(false);

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

  function handleShutdownSelectedNode() {
    if (!selectedNodeUrl) {
      setShutdownMessage("No node URL configured. Check VITE_NODE_URLS.");
      return;
    }

    setShowShutdownConfirm(true);
  }

  async function confirmShutdownSelectedNode() {
    if (!selectedNodeUrl) {
      setShutdownMessage("No node URL configured. Check VITE_NODE_URLS.");
      setShowShutdownConfirm(false);
      return;
    }

    setShutdownLoading(true);
    setShutdownMessage(null);

    try {
      const result = await fetchJson<ShutdownResponse>(`${selectedNodeUrl}/admin/shutdown`, {
        method: "POST",
      });
      setShutdownMessage(result.message || "Shutdown scheduled.");
      window.setTimeout(() => {
        void refreshStatus();
      }, 600);
    } catch (error) {
      setShutdownMessage(getErrorMessage(error));
    } finally {
      setShutdownLoading(false);
      setShowShutdownConfirm(false);
    }
  }

  useEffect(() => {
    void refreshStatus();
    void refreshLedger();
  }, [refreshStatus, refreshLedger]);

  return (
    <div className="relative">
      <ShutdownConfirmModal
        open={showShutdownConfirm}
        nodeUrl={selectedNodeUrl}
        loading={shutdownLoading}
        onCancel={() => setShowShutdownConfirm(false)}
        onConfirm={() => void confirmShutdownSelectedNode()}
      />

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
          <NodeStatusPanel
            nodeUrls={nodeUrls}
            selectedNodeIndex={selectedNodeIndex}
            onSelectNode={setSelectedNodeIndex}
            status={status}
            statusLoading={statusLoading}
            shutdownLoading={shutdownLoading}
            shutdownMessage={shutdownMessage}
            onRefreshStatus={() => void refreshStatus()}
            onRequestTerminate={handleShutdownSelectedNode}
          />

          <PaymentForm
            paymentId={paymentId}
            amount={amount}
            currency={currency}
            note={note}
            paymentLoading={paymentLoading}
            paymentError={paymentError}
            paymentResult={paymentResult}
            onPaymentIdChange={setPaymentId}
            onAmountChange={setAmount}
            onCurrencyChange={setCurrency}
            onNoteChange={setNote}
            onGeneratePaymentId={() => setPaymentId(generatePaymentId())}
            onSubmit={handleSubmitPayment}
          />

          <LedgerTable
            statusFilter={statusFilter}
            onStatusFilterChange={setStatusFilter}
            ledgerLoading={ledgerLoading}
            ledgerError={ledgerError}
            items={filteredLedgerItems}
            onRefreshLedger={() => void refreshLedger()}
          />
        </div>
      </div>
    </div>
  );
}

export default App;
