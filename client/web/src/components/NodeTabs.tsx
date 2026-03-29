type NodeTabsProps = {
  nodeUrls: string[];
  selectedNodeIndex: number;
  onSelectNode: (index: number) => void;
  nodeMetaByUrl?: Record<string, { nodeId?: string; role?: string }>;
};

function NodeTabs({ nodeUrls, selectedNodeIndex, onSelectNode, nodeMetaByUrl }: NodeTabsProps) {
  const selectedUrl = nodeUrls[selectedNodeIndex] ?? "";
  const selectedMeta = nodeMetaByUrl?.[selectedUrl];

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="text-base font-semibold text-slate-900">Node Selection</h2>
        <span className="text-xs text-slate-500">{nodeUrls.length} nodes</span>
      </div>
      <div className="flex gap-2 overflow-x-auto pb-1">
        {nodeUrls.map((url, index) => {
          const active = index === selectedNodeIndex;
          const meta = nodeMetaByUrl?.[url];
          const label = meta?.nodeId ? `Node ${meta.nodeId}` : `Node ${index + 1}`;
          const isLeader = meta?.role === "LEADER";
          const roleBadgeClass = isLeader
            ? "bg-indigo-100 text-indigo-700 border-indigo-200"
            : "bg-slate-100 text-slate-600 border-slate-200";
          return (
            <button
              key={url}
              type="button"
              onClick={() => onSelectNode(index)}
              className={`whitespace-nowrap rounded-full border px-3 py-1.5 text-sm font-medium transition ${
                active
                  ? "border-slate-900 bg-slate-900 text-white shadow-sm"
                  : "border-slate-300 bg-white text-slate-700 hover:border-slate-400 hover:bg-slate-50"
              }`}
              title={`${label} - ${url}`}
              aria-label={`Select node ${index + 1}`}
            >
              <span className="inline-flex items-center gap-1.5">
                {isLeader ? (
                  <svg viewBox="0 0 24 24" className="h-3.5 w-3.5" fill="currentColor" aria-hidden="true">
                    <path d="M5 18h14l-1.2-8.4-4 2.8L12 6l-1.8 6.4-4-2.8L5 18zm1.8 2a1 1 0 0 1 0-2h10.4a1 1 0 1 1 0 2H6.8z" />
                  </svg>
                ) : null}
                <span>{label}</span>
                {!active ? <span className={`rounded-full border px-1.5 py-0.5 text-[10px] leading-none ${roleBadgeClass}`}>{isLeader ? "Leader" : "Node"}</span> : null}
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
