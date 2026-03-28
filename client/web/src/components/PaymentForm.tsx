import { FormEvent } from "react";

type PaymentFormProps = {
  paymentId: string;
  amount: string;
  currency: string;
  paymentLoading: boolean;
  onPaymentIdChange: (value: string) => void;
  onAmountChange: (value: string) => void;
  onCurrencyChange: (value: string) => void;
  onGeneratePaymentId: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

function PaymentForm({
  paymentId,
  amount,
  currency,
  paymentLoading,
  onPaymentIdChange,
  onAmountChange,
  onCurrencyChange,
  onGeneratePaymentId,
  onSubmit,
}: PaymentFormProps) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <h2 className="mb-4 text-lg font-medium text-slate-900">Create Payment</h2>
      <form className="grid gap-4" onSubmit={onSubmit}>
        <div className="grid gap-2">
          <label htmlFor="payment-id" className="text-sm font-medium text-slate-700">
            Payment ID
          </label>
          <div className="relative">
            <input
              id="payment-id"
              value={paymentId}
              onChange={(event) => onPaymentIdChange(event.target.value)}
              className="w-full rounded-md border border-slate-300 px-3 py-2 pr-11 text-sm focus:border-slate-400 focus:outline-none"
              placeholder="Unique idempotency key"
            />
            <button
              type="button"
              onClick={onGeneratePaymentId}
              aria-label="Regenerate payment_id"
              title="Regenerate payment_id"
              className="absolute right-1.5 top-1/2 inline-flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-full border border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
            >
              <svg viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                <path d="M20 12a8 8 0 0 1-13.66 5.66" />
                <path d="M4 12a8 8 0 0 1 13.66-5.66" />
                <path d="M7 17H4v3" />
                <path d="M17 7h3V4" />
              </svg>
            </button>
          </div>
        </div>

        <div className="grid gap-2">
          <label htmlFor="amount" className="text-sm font-medium text-slate-700">
            Amount
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
            Currency
          </label>
          <input
            id="currency"
            value={currency}
            onChange={(event) => onCurrencyChange(event.target.value)}
            className="rounded-md border border-slate-300 px-3 py-2 text-sm uppercase focus:border-slate-400 focus:outline-none"
            placeholder="USD"
          />
        </div>

        <div className="flex justify-center">
          <button
            type="submit"
            disabled={paymentLoading}
            className="h-10 rounded-md bg-slate-900 px-6 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {paymentLoading ? "Submitting..." : "Submit Payment"}
          </button>
        </div>
        {paymentLoading ? <span className="text-sm text-slate-500">Please wait while the request is processed...</span> : null}
      </form>
    </section>
  );
}

export default PaymentForm;
