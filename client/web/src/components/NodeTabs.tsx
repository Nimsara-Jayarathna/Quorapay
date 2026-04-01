type NodeTabsProps = {
  nodeUrls: string[];
  selectedNodeIndex: number;
  onSelectNode: (index: number) => void;
  nodeMetaByUrl?: Record<string, { nodeId?: string; role?: string }>;
  showControls?: boolean;
  onOpenNodeManagement?: (action: "start" | "stop" | "restart") => void;
};

function NodeTabs({
  nodeUrls,
  selectedNodeIndex,
  onSelectNode,
  nodeMetaByUrl,
  showControls = false,
  onOpenNodeManagement,
}: NodeTabsProps) {
  const selectedUrl = nodeUrls[selectedNodeIndex] ?? "";
  const selectedMeta = nodeMetaByUrl?.[selectedUrl];

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-base font-semibold text-slate-900">Node Management</h2>
        {showControls && onOpenNodeManagement ? (
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => onOpenNodeManagement("start")}
              className="rounded-md border border-emerald-700 bg-emerald-700 px-3 py-1.5 text-sm font-semibold text-white hover:bg-emerald-600"
            >
              Start Node
            </button>
            <button
              type="button"
              onClick={() => onOpenNodeManagement("stop")}
              className="rounded-md border border-red-700 bg-red-700 px-3 py-1.5 text-sm font-semibold text-white hover:bg-red-600"
            >
              Terminate Node
            </button>
            <button
              type="button"
              onClick={() => onOpenNodeManagement("restart")}
              className="rounded-md border border-amber-700 bg-amber-700 px-3 py-1.5 text-sm font-semibold text-white hover:bg-amber-600"
            >
              Restart Node
            </button>
            <span className="ml-2 text-xs text-slate-500">{nodeUrls.length} nodes</span>
          </div>
        ) : (
          <span className="ml-2 text-xs text-slate-500">{nodeUrls.length} nodes</span>
        )}
      </div>
      <div className="flex flex-wrap items-center justify-center gap-2 pb-1">
        {nodeUrls.map((url, index) => {
          const active = index === selectedNodeIndex;
          const meta = nodeMetaByUrl?.[url];
          const label = meta?.nodeId ? `Node ${meta.nodeId}` : `Node ${index + 1}`;
          const isLeader = meta?.role === "LEADER";
          const roleBadgeClass = isLeader ? "bg-amber-100 text-amber-700 border-amber-200" : "bg-slate-100 text-slate-600 border-slate-200";
          return (
            <button
              key={url}
              type="button"
              onClick={() => onSelectNode(index)}
              className={`whitespace-nowrap rounded-full border px-3 py-1.5 text-sm font-medium transition ${
                active
                  ? isLeader
                    ? "border-amber-500 bg-amber-500 text-white shadow-sm"
                    : "border-slate-900 bg-slate-900 text-white shadow-sm"
                  : "border-slate-300 bg-white text-slate-700 hover:border-slate-400 hover:bg-slate-50"
              }`}
              title={`${label} - ${url}`}
              aria-label={`Select node ${index + 1}`}
            >
              <span className="inline-flex items-center gap-1.5">
                {isLeader ? (
                  <svg viewBox="0 0 24 24" className={`h-3.5 w-3.5 ${active ? "text-white" : "text-amber-600"}`} fill="currentColor" aria-hidden="true">
                    <path d="M5 18h14l-1.2-8.4-4 2.8L12 6l-1.8 6.4-4-2.8L5 18zm1.8 2a1 1 0 0 1 0-2h10.4a1 1 0 1 1 0 2H6.8z" />
                  </svg>
                ) : null}
                <span>{label}</span>
                <span className={`rounded-full border px-1.5 py-0.5 text-[10px] leading-none ${active ? "border-white/50 bg-white/20 text-white" : roleBadgeClass}`}>
                  {isLeader ? "Leader" : "Node"}
                </span>
              </span>
            </button>
          );
        })}
      </div>
      {selectedUrl ? (
        <div className="mt-3 rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600">
          <span className="font-medium text-slate-800">{selectedMeta?.nodeId ? `Node ${selectedMeta.nodeId}` : "Selected Node"}</span>
          <span className="mx-2 text-slate-400">•</span>
          <span className="font-mono">{selectedUrl}</span>
        </div>
      ) : null}
    </section>
  );
}

export default NodeTabs;
