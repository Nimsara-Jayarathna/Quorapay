import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";

import LedgerTable from "./components/LedgerTable";
import NodeStatusPanel from "./components/NodeStatusPanel";
import NodeTabs from "./components/NodeTabs";
import NodeManagementModal from "./components/NodeManagementModal";
import NodeActionModal from "./components/NodeActionModal";
import NoNodesConnectedModal from "./components/NoNodesConnectedModal";
import PaymentActionModal from "./components/PaymentActionModal";
import PaymentForm from "./components/PaymentForm";
import {
  AdminNodeActionResponse,
  fetchJson,
  fetchClusterNodes,
  getErrorMessage,
  LedgerResponse,
  NodeStatus,
  PaymentRequest,
  PaymentResponse,
  StatusFilter,
} from "./lib/api";

const configuredSeedNodeUrl = (import.meta.env.VITE_SEED_NODE_URL as string | undefined)?.trim() || "";
const configuredBasePort = Number(import.meta.env.VITE_NODE_BASE_PORT ?? "8001");
const basePort = Number.isInteger(configuredBasePort) && configuredBasePort > 0 ? configuredBasePort : 8001;
const configuredPortStep = Number(import.meta.env.VITE_NODE_PORT_STEP ?? "1");
const portStep = Number.isInteger(configuredPortStep) && configuredPortStep > 0 ? configuredPortStep : 1;
const configuredScanCount = Number(import.meta.env.VITE_NODE_SCAN_COUNT ?? "3");
const scanCount = Number.isInteger(configuredScanCount) && configuredScanCount > 0 ? configuredScanCount : 3;
const configuredHost = (import.meta.env.VITE_NODE_HOST as string | undefined)?.trim() || window.location.hostname || "localhost";
const generatedNodeUrls = Array.from({ length: scanCount }, (_, index) => `http://${configuredHost}:${basePort + index * portStep}`);
const initialNodeUrls = configuredSeedNodeUrl
  ? [configuredSeedNodeUrl, ...generatedNodeUrls.filter((url) => url !== configuredSeedNodeUrl)]
  : generatedNodeUrls;

const configuredDefaultIndex = Number(import.meta.env.VITE_DEFAULT_NODE_INDEX ?? "0");
const defaultNodeIndex =
  Number.isInteger(configuredDefaultIndex) && configuredDefaultIndex >= 0 && configuredDefaultIndex < initialNodeUrls.length
    ? configuredDefaultIndex
    : 0;
const configuredBlockingModalTimeoutMS = Number(import.meta.env.VITE_BLOCKING_MODAL_TIMEOUT_MS ?? "3000");
const blockingModalTimeoutMS =
  Number.isInteger(configuredBlockingModalTimeoutMS) && configuredBlockingModalTimeoutMS >= 0
    ? configuredBlockingModalTimeoutMS
    : 3000;
const adminApiBaseUrl = (import.meta.env.VITE_ADMIN_API_BASE_URL as string | undefined)?.trim() || "http://localhost:8090";

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
  const [nodeMetaLoaded, setNodeMetaLoaded] = useState(false);

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
  const shutdownMessageTimeoutRef = useRef<number | null>(null);
  const [adminToken, setAdminToken] = useState<string>(() => window.localStorage.getItem("quorapay_admin_token") ?? "");
  const [nodeManagementOpen, setNodeManagementOpen] = useState(false);
  const [nodeManagementAction, setNodeManagementAction] = useState<"start" | "stop" | "restart">("start");
  const [nodeManagementTargetNodeId, setNodeManagementTargetNodeId] = useState("A");
  const [nodeActionModalOpen, setNodeActionModalOpen] = useState(false);
  const [nodeActionModalState, setNodeActionModalState] = useState<"loading" | "success" | "error" | "info">("loading");
  const [nodeActionModalTitle, setNodeActionModalTitle] = useState("Applying Node Action");
  const [nodeActionModalMessage, setNodeActionModalMessage] = useState("Sending request...");
  const nodeActionModalTimeoutRef = useRef<number | null>(null);

  const filteredLedgerItems = useMemo(() => {
    if (statusFilter === "ALL") {
      return ledgerItems;
    }
    return ledgerItems.filter((item) => item.status === statusFilter);
  }, [ledgerItems, statusFilter]);

  const refreshTopology = useCallback(async () => {
    const seeds = Array.from(new Set([...nodeUrls, ...generatedNodeUrls]));
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
      setStatusError("Node unreachable: no node URL configured. Check VITE_SEED_NODE_URL and scan settings.");
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
    setNodeMetaLoaded(true);
  }, [nodeUrls]);

  const connectedNodeCount = useMemo(
    () => Object.values(nodeMetaByUrl).filter((meta) => Boolean(meta.nodeId)).length,
    [nodeMetaByUrl],
  );
  const noNodesConnected = nodeUrls.length === 0 || (nodeMetaLoaded && connectedNodeCount === 0);

  const refreshLedger = useCallback(async () => {
    if (!selectedNodeUrl) {
      setLedgerItems([]);
      setLedgerError("No node URL configured. Check VITE_SEED_NODE_URL and scan settings.");
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
        }, blockingModalTimeoutMS);
      }
    };

    if (!selectedNodeUrl) {
      const message = "No node URL configured. Check VITE_SEED_NODE_URL and scan settings.";
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

  const selectedNodeId =
    status?.node_id ||
    nodeMetaByUrl[selectedNodeUrl]?.nodeId ||
    "";

  const knownNodeIds = useMemo(
    () =>
      Array.from(
        new Set(
          Object.values(nodeMetaByUrl)
            .map((meta) => (meta.nodeId || "").trim().toUpperCase())
            .filter(Boolean),
        ),
      ),
    [nodeMetaByUrl],
  );

  useEffect(() => {
    if (nodeManagementAction === "start") {
      const firstStartable = Array.from({ length: 26 }, (_, idx) => String.fromCharCode(65 + idx)).find((id) => !knownNodeIds.includes(id));
      if (!firstStartable) {
        setNodeManagementTargetNodeId("A");
        return;
      }
      if (!nodeManagementTargetNodeId || knownNodeIds.includes(nodeManagementTargetNodeId)) {
        setNodeManagementTargetNodeId(firstStartable);
      }
      return;
    }
    if (knownNodeIds.length > 0 && !knownNodeIds.includes(nodeManagementTargetNodeId)) {
      setNodeManagementTargetNodeId(knownNodeIds[0]);
    }
  }, [knownNodeIds, nodeManagementAction, nodeManagementTargetNodeId]);

  const openNodeManagement = useCallback(
    (action: "start" | "stop" | "restart") => {
      setNodeManagementAction(action);
      if (action === "start") {
        const firstStartable = Array.from({ length: 26 }, (_, idx) => String.fromCharCode(65 + idx)).find((id) => !knownNodeIds.includes(id));
        setNodeManagementTargetNodeId(firstStartable || "A");
      } else {
        setNodeManagementTargetNodeId(selectedNodeId || knownNodeIds[0] || "");
      }
      setNodeManagementOpen(true);
    },
    [knownNodeIds, selectedNodeId],
  );

  async function handleNodeAction(action: "start" | "stop" | "restart", nodeId: string) {
    const targetNodeId = nodeId.trim().toUpperCase();
    const actionLabel = action === "start" ? "Start" : action === "stop" ? "Terminate" : "Restart";
    const showNodeActionModal = (state: "loading" | "success" | "error" | "info", title: string, message: string) => {
      if (nodeActionModalTimeoutRef.current !== null) {
        window.clearTimeout(nodeActionModalTimeoutRef.current);
        nodeActionModalTimeoutRef.current = null;
      }
      setNodeActionModalState(state);
      setNodeActionModalTitle(title);
      setNodeActionModalMessage(message);
      setNodeActionModalOpen(true);
      if (state !== "loading") {
        nodeActionModalTimeoutRef.current = window.setTimeout(() => {
          setNodeActionModalOpen(false);
          nodeActionModalTimeoutRef.current = null;
        }, blockingModalTimeoutMS);
      }
    };

    if (!targetNodeId) {
      setShutdownMessage("Node ID is required.");
      return;
    }
    if (!/^[A-Z]$/.test(targetNodeId)) {
      setShutdownMessage("Node ID must be a single letter (A-Z).");
      return;
    }
    if (action !== "start" && knownNodeIds.length > 0 && !knownNodeIds.includes(targetNodeId)) {
      setShutdownMessage(`Unknown node ID: ${targetNodeId}.`);
      return;
    }
    if (!adminToken.trim()) {
      setShutdownMessage("Admin token is required.");
      return;
    }

    setShutdownLoading(true);
    showNodeActionModal("loading", `${actionLabel} Node`, `Applying ${action} on node ${targetNodeId}...`);
    setNodeManagementOpen(false);
    if (shutdownMessageTimeoutRef.current !== null) {
      window.clearTimeout(shutdownMessageTimeoutRef.current);
      shutdownMessageTimeoutRef.current = null;
    }
    setShutdownMessage(null);

    try {
      const result = await fetchJson<AdminNodeActionResponse>(`${adminApiBaseUrl}/admin/node/${targetNodeId}/${action}`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${adminToken.trim()}`,
        },
      });
      setShutdownMessage(`Node ${result.node_id} ${result.action} requested.`);
      showNodeActionModal("success", `${actionLabel} Requested`, `Node ${result.node_id} ${result.action} requested.`);
      shutdownMessageTimeoutRef.current = window.setTimeout(() => {
        setShutdownMessage(null);
        shutdownMessageTimeoutRef.current = null;
      }, 3000);
      window.setTimeout(() => {
        void refreshStatus();
        void refreshNodeMeta();
      }, 600);
    } catch (error) {
      const message = getErrorMessage(error);
      setShutdownMessage(message);
      showNodeActionModal("error", `${actionLabel} Failed`, message);
    } finally {
      setShutdownLoading(false);
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
      if (nodeActionModalTimeoutRef.current !== null) {
        window.clearTimeout(nodeActionModalTimeoutRef.current);
      }
    },
    [],
  );

  return (
    <div className="relative">
      <PaymentActionModal
        open={paymentModalOpen}
        state={paymentModalState}
        title={paymentModalTitle}
        message={paymentModalMessage}
      />
      <NodeManagementModal
        open={nodeManagementOpen}
        action={nodeManagementAction}
        targetNodeId={nodeManagementTargetNodeId}
        knownNodeIds={knownNodeIds}
        adminToken={adminToken}
        loading={shutdownLoading}
        message={shutdownMessage}
        onClose={() => setNodeManagementOpen(false)}
        onTargetNodeIdChange={(value) => setNodeManagementTargetNodeId(value.trim().toUpperCase())}
        onAdminTokenChange={(value) => {
          setAdminToken(value);
          window.localStorage.setItem("quorapay_admin_token", value);
        }}
        onConfirm={() => void handleNodeAction(nodeManagementAction, nodeManagementTargetNodeId)}
      />
      <NodeActionModal
        open={nodeActionModalOpen}
        state={nodeActionModalState}
        title={nodeActionModalTitle}
        message={nodeActionModalMessage}
      />
      <NoNodesConnectedModal
        open={noNodesConnected}
        onRetry={() => {
          void refreshTopology();
          void refreshNodeMeta();
          void refreshStatus();
        }}
      />

      <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <header className="mb-6">
          <h1 className="text-2xl font-semibold text-slate-900">Quorapay Web Client</h1>
          <p className="mt-1 text-sm text-slate-600">Submit payments, inspect node status, and view replicated ledger data.</p>
        </header>

        {statusError ? (
          <div className="mb-6 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">{statusError}</div>
        ) : null}

        <div className="space-y-6">
          <NodeTabs
            nodeUrls={nodeUrls}
            selectedNodeIndex={selectedNodeIndex}
            onSelectNode={setSelectedNodeIndex}
            nodeMetaByUrl={nodeMetaByUrl}
            onOpenNodeManagement={openNodeManagement}
          />

          <div className="grid items-stretch gap-6 xl:grid-cols-12">
            <div className="xl:col-span-7">
              <NodeStatusPanel
                status={status}
                statusLoading={statusLoading}
                nodeActionLoading={shutdownLoading}
                shutdownMessage={shutdownMessage}
                selectedNodeId={selectedNodeId}
                onRefreshStatus={() => void refreshStatus()}
                onNodeAction={(action, nodeId) => void handleNodeAction(action, nodeId)}
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
