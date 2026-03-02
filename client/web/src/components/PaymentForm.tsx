import { FormEvent } from "react";

import { PaymentResponse } from "../lib/api";

type PaymentFormProps = {
  paymentId: string;
  amount: string;
  currency: string;
  note: string;
  paymentLoading: boolean;
  paymentError: string | null;
  paymentResult: PaymentResponse | null;
  onPaymentIdChange: (value: string) => void;
  onAmountChange: (value: string) => void;
  onCurrencyChange: (value: string) => void;
  onNoteChange: (value: string) => void;
  onGeneratePaymentId: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

function PaymentForm({
  paymentId,
  amount,
  currency,
  note,
  paymentLoading,
  paymentError,
  paymentResult,
  onPaymentIdChange,
  onAmountChange,
  onCurrencyChange,
  onNoteChange,
  onGeneratePaymentId,
  onSubmit,
}: PaymentFormProps) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <h2 className="mb-4 text-lg font-medium text-slate-900">Create Payment</h2>
      <form className="grid gap-4" onSubmit={onSubmit}>
        <div className="grid gap-2">
          <label htmlFor="payment-id" className="text-sm font-medium text-slate-700">
            payment_id
          </label>
          <div className="flex flex-col gap-2 sm:flex-row">
            <input
              id="payment-id"
              value={paymentId}
              onChange={(event) => onPaymentIdChange(event.target.value)}
              className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
              placeholder="Unique idempotency key"
            />
            <button
              type="button"
              onClick={onGeneratePaymentId}
              className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              Generate payment_id
            </button>
          </div>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="grid gap-2">
            <label htmlFor="amount" className="text-sm font-medium text-slate-700">
              amount
            </label>
            <input
              id="amount"
              type="number"
              min="0"
              step="0.01"
              value={amount}
              onChange={(event) => onAmountChange(event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
            />
          </div>
          <div className="grid gap-2">
            <label htmlFor="currency" className="text-sm font-medium text-slate-700">
              currency
            </label>
            <input
              id="currency"
              value={currency}
              onChange={(event) => onCurrencyChange(event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm uppercase focus:border-slate-400 focus:outline-none"
              placeholder="USD"
            />
          </div>
        </div>

        <div className="grid gap-2">
          <label htmlFor="note" className="text-sm font-medium text-slate-700">
            note (optional)
          </label>
          <input
            id="note"
            value={note}
            onChange={(event) => onNoteChange(event.target.value)}
            className="rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
            placeholder="Optional note"
          />
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <button
            type="submit"
            disabled={paymentLoading}
            className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {paymentLoading ? "Submitting..." : "Submit Payment"}
          </button>
          {paymentError ? <span className="text-sm text-red-700">{paymentError}</span> : null}
          {paymentResult ? <span className="text-sm text-slate-600">{`${paymentResult.status}: ${paymentResult.message}`}</span> : null}
        </div>
      </form>

      <div className="mt-4">
        <h3 className="text-sm font-medium text-slate-700">Response</h3>
        <pre className="mt-2 max-h-72 overflow-auto rounded-md bg-slate-900 p-3 font-mono text-xs text-slate-100">
          {paymentResult ? JSON.stringify(paymentResult, null, 2) : "No response yet."}
        </pre>
      </div>
    </section>
  );
}

export default PaymentForm;
