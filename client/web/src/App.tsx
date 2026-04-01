import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";

import LedgerTable from "./components/LedgerTable";
import NodeStatusPanel from "./components/NodeStatusPanel";
import NodeTabs from "./components/NodeTabs";
import NodeManagementModal from "./components/NodeManagementModal";
import NodeActionModal from "./components/NodeActionModal";
import NoNodesConnectedModal from "./components/NoNodesConnectedModal";
import PaymentActionModal from "./components/PaymentActionModal";
import PaymentForm from "./components/PaymentForm";
import ClientStripeCheckout from "./components/ClientStripeCheckout";
import {
  AdminNodeActionResponse,
  fetchJson,
  fetchClusterNodes,
  getErrorMessage,
  LedgerResponse,
  NodeStatus,
  PaymentEventsResponse,
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

function resolveRoute(pathname: string): "/admin" | "/client" | "not-found" {
  if (pathname === "/admin" || pathname === "/") {
    return "/admin";
  }
  if (pathname === "/client") {
    return "/client";
  }
  return "not-found";
}

function stageMessageFromEvent(stage: string): string {
  switch (stage) {
    case "RECEIVED":
      return "Stage 1/6: Payment request received by selected node.";
    case "FORWARDED_TO_LEADER":
      return "Stage 2/6: Request forwarded to leader node.";
    case "LEADER_PROCESSING":
      return "Stage 3/6: Leader validating and processing Stripe payment.";
    case "LEADER_RESPONSE_SUCCESS":
      return "Stage 4/6: Leader returned successful processing response.";
    case "LEADER_RESPONSE_FAILED":
      return "Stage 4/6: Leader returned failed processing response.";
    case "COMMITTED":
      return "Stage 6/6: Payment committed and replicated.";
    case "PROVIDER_FAILED":
      return "Stage 5/6: Payment provider rejected transaction.";
    case "QUORUM_NOT_REACHED":
      return "Stage 5/6: Replication quorum not reached.";
    case "REPLICATION_FAILED":
      return "Stage 5/6: Replication failed during cluster write.";
    default:
      return "Processing payment across cluster nodes...";
  }
}

function App() {
  const [route, setRoute] = useState<"/admin" | "/client" | "not-found">(() => resolveRoute(window.location.pathname));

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
  const [simulateOutcome, setSimulateOutcome] = useState<"SUCCESS" | "FAILED">("SUCCESS");
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
  const [eventItems, setEventItems] = useState<Array<{ timestamp: string; stage: string; message: string; payment_id?: string }>>([]);
  const [eventsError, setEventsError] = useState<string | null>(null);

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

  const refreshEvents = useCallback(async () => {
    if (!selectedNodeUrl) {
      return;
    }
    try {
      const result = await fetchJson<PaymentEventsResponse>(`${selectedNodeUrl}/events`);
      setEventItems(result.items ?? []);
      setEventsError(null);
    } catch (error) {
      setEventItems([]);
      setEventsError(getErrorMessage(error));
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
      simulate_outcome: simulateOutcome,
    };

    const requestedPaymentID = paymentId.trim();
    setPaymentLoading(true);
    setPaymentError(null);
    showPaymentModal("loading", "Processing Payment", "Stage 1/6: Payment request received by selected node.");

    let pollActive = true;
    let lastStage = "";
    const pollEvents = async () => {
      try {
        const events = await fetchJson<PaymentEventsResponse>(`${selectedNodeUrl}/events?payment_id=${encodeURIComponent(requestedPaymentID)}`);
        const latest = events.items?.[events.items.length - 1];
        if (latest && latest.stage !== lastStage) {
          lastStage = latest.stage;
          showPaymentModal("loading", "Processing Payment", stageMessageFromEvent(latest.stage));
        }
      } catch {
        // ignore polling failures while payment request is in progress
      }
    };
    void pollEvents();
    const pollTimer = window.setInterval(() => {
      if (!pollActive) {
        return;
      }
      void pollEvents();
    }, 700);

    try {
      const result = await fetchJson<PaymentResponse>(`${selectedNodeUrl}/pay`, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      pollActive = false;
      window.clearInterval(pollTimer);
      setPaymentResult(result);
      const trace = result.trace;
      const summary = [
        `Stage 6/6: Completed successfully`,
        `Payment ID: ${result.payment_id}`,
        `Leader: ${result.leader_id ?? "-"}`,
        `Routed To Leader: ${trace?.routed_to_leader ? "Yes" : "No"}`,
        `Quorum: ${trace?.ack_count ?? "-"}/${trace?.required_quorum ?? "-"}`,
      ].join("\n");
      showPaymentModal("success", "Payment Accepted", summary);
      void refreshStatus();
      void refreshLedger();
      void refreshEvents();
    } catch (error) {
      pollActive = false;
      window.clearInterval(pollTimer);
      setPaymentResult(null);
      const message = getErrorMessage(error);
      setPaymentError(message);
      showPaymentModal("error", "Payment Failed", `Stage 6/6: Failed\n${message}`);
      void refreshStatus();
      void refreshLedger();
      void refreshEvents();
    } finally {
      pollActive = false;
      window.clearInterval(pollTimer);
      setPaymentLoading(false);
    }
  }

  const selectedNodeId = status?.node_id || nodeMetaByUrl[selectedNodeUrl]?.nodeId || "";

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
    void refreshEvents();
  }, [refreshStatus, refreshLedger, refreshEvents]);

  useEffect(() => {
    void refreshNodeMeta();
    const intervalId = window.setInterval(() => {
      void refreshNodeMeta();
      void refreshEvents();
    }, 5000);
    return () => window.clearInterval(intervalId);
  }, [refreshNodeMeta, refreshEvents]);

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

  useEffect(() => {
    const onPopState = () => {
      setRoute(resolveRoute(window.location.pathname));
    };
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

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

  const navigate = (target: "/admin" | "/client") => {
    if (window.location.pathname !== target) {
      window.history.pushState({}, "", target);
      setRoute(resolveRoute(target));
    }
  };

  if (route === "not-found") {
    return (
      <div className="mx-auto max-w-4xl px-4 py-10 sm:px-6 lg:px-8">
        <div className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
          <h1 className="text-xl font-semibold text-slate-900">Route Not Found</h1>
          <p className="mt-2 text-sm text-slate-600">Only `/admin` and `/client` are valid routes in this prototype.</p>
          <div className="mt-4 flex gap-2">
            <button
              type="button"
              onClick={() => navigate("/admin")}
              className="rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white hover:bg-slate-800"
            >
              Go to Admin
            </button>
            <button
              type="button"
              onClick={() => navigate("/client")}
              className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              Go to Client
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="relative">
      <PaymentActionModal open={paymentModalOpen} state={paymentModalState} title={paymentModalTitle} message={paymentModalMessage} />
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
      <NodeActionModal open={nodeActionModalOpen} state={nodeActionModalState} title={nodeActionModalTitle} message={nodeActionModalMessage} />
      <NoNodesConnectedModal
        open={noNodesConnected}
        onRetry={() => {
          void refreshTopology();
          void refreshNodeMeta();
          void refreshStatus();
          void refreshEvents();
        }}
      />

      <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <header className="mb-6 flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-2xl font-semibold text-slate-900">Quorapay Prototype</h1>
            <p className="mt-1 text-sm text-slate-600">Distributed payment simulation with selected-node perspective and process logs.</p>
          </div>
          <div className="inline-flex rounded-md border border-slate-300 bg-white p-1">
            <button
              type="button"
              className={`rounded px-3 py-1.5 text-sm font-medium ${route === "/admin" ? "bg-slate-900 text-white" : "text-slate-700"}`}
              onClick={() => navigate("/admin")}
            >
              Admin UI
            </button>
            <button
              type="button"
              className={`rounded px-3 py-1.5 text-sm font-medium ${route === "/client" ? "bg-slate-900 text-white" : "text-slate-700"}`}
              onClick={() => navigate("/client")}
            >
              Client UI
            </button>
          </div>
        </header>

        {statusError ? <div className="mb-6 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">{statusError}</div> : null}

        <div className="space-y-6">
          <NodeTabs
            nodeUrls={nodeUrls}
            selectedNodeIndex={selectedNodeIndex}
            onSelectNode={setSelectedNodeIndex}
            nodeMetaByUrl={nodeMetaByUrl}
            showControls={route === "/admin"}
            onOpenNodeManagement={route === "/admin" ? openNodeManagement : undefined}
          />

          {route === "/client" ? (
            <div className="mx-auto max-w-3xl">
              <ClientStripeCheckout
                amount={amount}
                currency={currency}
                paymentLoading={paymentLoading}
                onAmountChange={setAmount}
                onCurrencyChange={setCurrency}
                onSimulateOutcomeChange={setSimulateOutcome}
                onSubmit={handleSubmitPayment}
              />
              {paymentError ? <div className="mt-3 rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700">{paymentError}</div> : null}
              {paymentResult ? (
                <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
                  Payment {paymentResult.payment_id} processed by leader {paymentResult.leader_id ?? "-"}, quorum {paymentResult.trace?.ack_count ?? "-"}/{paymentResult.trace?.required_quorum ?? "-"}.
                </div>
              ) : null}
            </div>
          ) : (
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
                  simulateOutcome={simulateOutcome}
                  paymentLoading={paymentLoading}
                  onPaymentIdChange={setPaymentId}
                  onAmountChange={setAmount}
                  onCurrencyChange={setCurrency}
                  onSimulateOutcomeChange={setSimulateOutcome}
                  onGeneratePaymentId={() => setPaymentId(generatePaymentId())}
                  onSubmit={handleSubmitPayment}
                />
              </div>
            </div>
          )}

          {route === "/admin" ? (
            <>
              <LedgerTable
                statusFilter={statusFilter}
                onStatusFilterChange={setStatusFilter}
                ledgerLoading={ledgerLoading}
                ledgerError={ledgerError}
                items={filteredLedgerItems}
                onRefreshLedger={() => void refreshLedger()}
              />

              <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
                <div className="mb-3 flex items-center justify-between gap-2">
                  <h2 className="text-lg font-medium text-slate-900">Payment Process Logs (Selected Node)</h2>
                  <button
                    type="button"
                    onClick={() => void refreshEvents()}
                    className="rounded-md bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-800"
                  >
                    Refresh Logs
                  </button>
                </div>
                {eventsError ? <div className="mb-2 text-sm text-red-700">Log error: {eventsError}</div> : null}
                <div className="max-h-72 overflow-auto rounded-md border border-slate-200">
                  <table className="min-w-full divide-y divide-slate-200 text-sm">
                    <thead className="bg-slate-50">
                      <tr>
                        <th className="px-3 py-2 text-left font-medium text-slate-600">Timestamp</th>
                        <th className="px-3 py-2 text-left font-medium text-slate-600">Stage</th>
                        <th className="px-3 py-2 text-left font-medium text-slate-600">Payment</th>
                        <th className="px-3 py-2 text-left font-medium text-slate-600">Message</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 bg-white">
                      {eventItems.length === 0 ? (
                        <tr>
                          <td className="px-3 py-4 text-slate-500" colSpan={4}>
                            No process logs.
                          </td>
                        </tr>
                      ) : (
                        eventItems.slice().reverse().map((item, idx) => (
                          <tr key={`${item.timestamp}-${item.stage}-${idx}`}>
                            <td className="px-3 py-2 text-slate-700">{new Date(item.timestamp).toLocaleString()}</td>
                            <td className="px-3 py-2 text-slate-700">{item.stage}</td>
                            <td className="px-3 py-2 font-mono text-xs text-slate-700">{item.payment_id ?? "-"}</td>
                            <td className="px-3 py-2 text-slate-700">{item.message}</td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </section>
            </>
          ) : null}
        </div>
      </div>
    </div>
  );
}

export default App;
