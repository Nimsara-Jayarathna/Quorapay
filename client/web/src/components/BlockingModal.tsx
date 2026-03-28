import { ReactNode } from "react";

type BlockingModalProps = {
  open: boolean;
  children: ReactNode;
};

function BlockingModal({ open, children }: BlockingModalProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/45 px-4 py-6 backdrop-blur-md">
      <div className="w-full max-w-lg rounded-3xl border border-white/50 bg-white/90 p-6 shadow-2xl sm:p-7">
        {children}
      </div>
    </div>
  );
}

export default BlockingModal;
