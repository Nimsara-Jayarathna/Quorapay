import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";

import LedgerTable from "./components/LedgerTable";
import NodeStatusPanel from "./components/NodeStatusPanel";
import NodeTabs from "./components/NodeTabs";
import PaymentActionModal from "./components/PaymentActionModal";
import PaymentForm from "./components/PaymentForm";
import ShutdownConfirmModal from "./components/ShutdownConfirmModal";
import {
  fetchJson,
  fetchClusterNodes,
  getErrorMessage,
  LedgerResponse,
  NodeStatus,
  PaymentRequest,
  PaymentResponse,
  ShutdownResponse,
  StatusFilter,
} from "./lib/api";

const configuredNodeUrls = (import.meta.env.VITE_NODE_URLS as string | undefined)
  ?.split(",")
  .map((url) => url.trim())
  .filter(Boolean) ?? [];

const configuredClusterSize = Number(import.meta.env.VITE_CLUSTER_SIZE ?? "3");
const clusterSize = Number.isInteger(configuredClusterSize) && configuredClusterSize > 0 ? configuredClusterSize : 3;
const configuredBasePort = Number(import.meta.env.VITE_NODE_BASE_PORT ?? "8001");
const basePort = Number.isInteger(configuredBasePort) && configuredBasePort > 0 ? configuredBasePort : 8001;
const configuredHost = (import.meta.env.VITE_NODE_HOST as string | undefined)?.trim() || window.location.hostname || "localhost";
const generatedNodeUrls = Array.from({ length: clusterSize }, (_, index) => `http://${configuredHost}:${basePort + index}`);
const initialNodeUrls = configuredNodeUrls.length > 0 ? configuredNodeUrls : generatedNodeUrls;

const configuredDefaultIndex = Number(import.meta.env.VITE_DEFAULT_NODE_INDEX ?? "0");
const defaultNodeIndex =
  Number.isInteger(configuredDefaultIndex) && configuredDefaultIndex >= 0 && configuredDefaultIndex < initialNodeUrls.length
    ? configuredDefaultIndex
    : 0;

function generatePaymentId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `pay-${Date.now()}-${Math.random().toString(16).slice(2, 10)}`;
}

function App() {
  const [nodeUrls, setNodeUrls] = useState<string[]>(initialNodeUrls);
  const [selectedNodeIndex, setSelectedNodeIndex] = useState(defaultNodeIndex);
  const selectedNodeUrl = nodeUrls[selectedNodeIndex] ?? "";
  const [nodeMetaByUrl, setNodeMetaByUrl] = useState<Record<string, { nodeId?: string; role?: string }>>({});

  const [status, setStatus] = useState<NodeStatus | null>(null);
  const [statusLoading, setStatusLoading] = useState(false);
  const [statusError, setStatusError] = useState<string | null>(null);

  const [paymentId, setPaymentId] = useState(generatePaymentId());
  const [amount, setAmount] = useState("10.00");
  const [currency, setCurrency] = useState("USD");
  const [paymentLoading, setPaymentLoading] = useState(false);
  const [paymentError, setPaymentError] = useState<string | null>(null);
  const [paymentResult, setPaymentResult] = useState<PaymentResponse | null>(null);
  const [paymentModalOpen, setPaymentModalOpen] = useState(false);
  const [paymentModalState, setPaymentModalState] = useState<"loading" | "success" | "error" | "info">("loading");
  const [paymentModalTitle, setPaymentModalTitle] = useState("Processing Payment");
  const [paymentModalMessage, setPaymentModalMessage] = useState("Submitting payment to cluster...");
  const paymentModalTimeoutRef = useRef<number | null>(null);

  const [ledgerItems, setLedgerItems] = useState<LedgerResponse["items"]>([]);
  const [ledgerLoading, setLedgerLoading] = useState(false);
  const [ledgerError, setLedgerError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("ALL");

  const [shutdownLoading, setShutdownLoading] = useState(false);
  const [shutdownMessage, setShutdownMessage] = useState<string | null>(null);
  const [showShutdownConfirm, setShowShutdownConfirm] = useState(false);
  const shutdownMessageTimeoutRef = useRef<number | null>(null);

  const filteredLedgerItems = useMemo(() => {
    if (statusFilter === "ALL") {
      return ledgerItems;
    }
    return ledgerItems.filter((item) => item.status === statusFilter);
  }, [ledgerItems, statusFilter]);

  const refreshTopology = useCallback(async () => {
    const seeds = Array.from(new Set([...nodeUrls, ...configuredNodeUrls, ...generatedNodeUrls]));
    for (const seed of seeds) {
      try {
        const discoveredUrls = await fetchClusterNodes(seed);
        if (discoveredUrls.length > 0) {
          setNodeUrls((prev) => {
            const next = Array.from(new Set(discoveredUrls)).sort();
            if (prev.length === next.length && prev.every((value, idx) => value === next[idx])) {
              return prev;
            }
            return next;
          });
          return;
        }
      } catch {
        // try next seed
      }
    }
  }, [nodeUrls]);

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

  const refreshNodeMeta = useCallback(async () => {
    const next: Record<string, { nodeId?: string; role?: string }> = {};
    await Promise.all(
      nodeUrls.map(async (url) => {
        try {
          const data = await fetchJson<NodeStatus>(`${url}/status`);
          next[url] = { nodeId: data.node_id, role: data.role };
        } catch {
          next[url] = {};
        }
      }),
    );
    setNodeMetaByUrl(next);
  }, [nodeUrls]);

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

    const showPaymentModal = (state: "loading" | "success" | "error" | "info", title: string, message: string) => {
      if (paymentModalTimeoutRef.current !== null) {
        window.clearTimeout(paymentModalTimeoutRef.current);
        paymentModalTimeoutRef.current = null;
      }
      setPaymentModalState(state);
      setPaymentModalTitle(title);
      setPaymentModalMessage(message);
      setPaymentModalOpen(true);
      if (state !== "loading") {
        paymentModalTimeoutRef.current = window.setTimeout(() => {
          setPaymentModalOpen(false);
          paymentModalTimeoutRef.current = null;
        }, 3000);
      }
    };

    if (!selectedNodeUrl) {
      const message = "No node URL configured. Check VITE_NODE_URLS.";
      setPaymentError(message);
      showPaymentModal("error", "Payment Failed", message);
      return;
    }

    const numericAmount = Number(amount);
    if (!paymentId.trim()) {
      const message = "payment_id is required.";
      setPaymentError(message);
      showPaymentModal("error", "Payment Validation Error", message);
      return;
    }
    if (!Number.isFinite(numericAmount) || numericAmount <= 0) {
      const message = "amount must be a positive number.";
      setPaymentError(message);
      showPaymentModal("error", "Payment Validation Error", message);
      return;
    }
    if (!currency.trim()) {
      const message = "currency is required.";
      setPaymentError(message);
      showPaymentModal("error", "Payment Validation Error", message);
      return;
    }

    const payload: PaymentRequest = {
      payment_id: paymentId.trim(),
      amount: numericAmount,
      currency: currency.trim().toUpperCase(),
    };

    setPaymentLoading(true);
    setPaymentError(null);
    showPaymentModal("loading", "Processing Payment", "Submitting payment to cluster...");

    try {
      const result = await fetchJson<PaymentResponse>(`${selectedNodeUrl}/pay`, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      setPaymentResult(result);
      const responseMessage = `${result.status}: ${result.message ?? "Request completed successfully."}`;
      if ((result.message ?? "").toLowerCase().includes("already processed")) {
        showPaymentModal("info", "Payment Already Processed", responseMessage);
      } else {
        showPaymentModal("success", "Payment Accepted", responseMessage);
      }
      void refreshStatus();
      void refreshLedger();
    } catch (error) {
      setPaymentResult(null);
      const message = getErrorMessage(error);
      setPaymentError(message);
      showPaymentModal("error", "Payment Failed", message);
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
    if (shutdownMessageTimeoutRef.current !== null) {
      window.clearTimeout(shutdownMessageTimeoutRef.current);
      shutdownMessageTimeoutRef.current = null;
    }
    setShutdownMessage(null);

    try {
      const result = await fetchJson<ShutdownResponse>(`${selectedNodeUrl}/admin/shutdown`, {
        method: "POST",
      });
      setShutdownMessage(result.message || "Shutdown scheduled.");
      shutdownMessageTimeoutRef.current = window.setTimeout(() => {
        setShutdownMessage(null);
        shutdownMessageTimeoutRef.current = null;
      }, 3000);
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

  useEffect(() => {
    void refreshNodeMeta();
    const intervalId = window.setInterval(() => {
      void refreshNodeMeta();
    }, 5000);
    return () => window.clearInterval(intervalId);
  }, [refreshNodeMeta]);

  useEffect(() => {
    void refreshTopology();
    const intervalId = window.setInterval(() => {
      void refreshTopology();
    }, 5000);
    return () => window.clearInterval(intervalId);
  }, [refreshTopology]);

  useEffect(() => {
    if (selectedNodeIndex >= nodeUrls.length) {
      setSelectedNodeIndex(0);
    }
  }, [nodeUrls, selectedNodeIndex]);

  useEffect(
    () => () => {
      if (paymentModalTimeoutRef.current !== null) {
        window.clearTimeout(paymentModalTimeoutRef.current);
      }
      if (shutdownMessageTimeoutRef.current !== null) {
        window.clearTimeout(shutdownMessageTimeoutRef.current);
      }
    },
    [],
  );

  return (
    <div className="relative">
      <ShutdownConfirmModal
        open={showShutdownConfirm}
        nodeUrl={selectedNodeUrl}
        loading={shutdownLoading}
        onCancel={() => setShowShutdownConfirm(false)}
        onConfirm={() => void confirmShutdownSelectedNode()}
      />
      <PaymentActionModal
        open={paymentModalOpen}
        state={paymentModalState}
        title={paymentModalTitle}
        message={paymentModalMessage}
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
          <NodeTabs
            nodeUrls={nodeUrls}
            selectedNodeIndex={selectedNodeIndex}
            onSelectNode={setSelectedNodeIndex}
            nodeMetaByUrl={nodeMetaByUrl}
          />

          <div className="grid items-stretch gap-6 xl:grid-cols-12">
            <div className="xl:col-span-7">
              <NodeStatusPanel
                status={status}
                statusLoading={statusLoading}
                shutdownLoading={shutdownLoading}
                shutdownMessage={shutdownMessage}
                onRefreshStatus={() => void refreshStatus()}
                onRequestTerminate={handleShutdownSelectedNode}
              />
            </div>

            <div className="xl:col-span-5">
              <PaymentForm
                paymentId={paymentId}
                amount={amount}
                currency={currency}
                paymentLoading={paymentLoading}
                onPaymentIdChange={setPaymentId}
                onAmountChange={setAmount}
                onCurrencyChange={setCurrency}
                onGeneratePaymentId={() => setPaymentId(generatePaymentId())}
                onSubmit={handleSubmitPayment}
              />
            </div>
          </div>

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
