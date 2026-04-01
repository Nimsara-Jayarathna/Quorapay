import { FormEvent } from "react";

type ClientStripeCheckoutProps = {
  amount: string;
  currency: string;
  paymentLoading: boolean;
  currencyOptions: string[];
  onAmountChange: (value: string) => void;
  onCurrencyChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

function ClientStripeCheckout({
  amount,
  currency,
  paymentLoading,
  currencyOptions,
  onAmountChange,
  onCurrencyChange,
  onSubmit,
}: ClientStripeCheckoutProps) {
  return (
    <section className="h-full rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
      <div className="mb-5 flex items-start justify-between gap-3">
        <div>
          <h2 className="text-xl font-semibold text-slate-900">Stripe Checkout</h2>
          <p className="mt-1 text-sm text-slate-600">Secure Stripe-hosted card collection with distributed finalization.</p>
        </div>
        <span className="inline-flex items-center rounded-full border border-indigo-200 bg-indigo-50 px-2.5 py-1 text-xs font-semibold uppercase tracking-wide text-indigo-700">
          Client
        </span>
      </div>

      <form className="grid gap-4" onSubmit={onSubmit}>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div className="grid gap-2">
            <label htmlFor="checkout-amount" className="text-sm font-semibold text-slate-700">Amount</label>
            <input
              id="checkout-amount"
              type="number"
              min="0"
              step="0.01"
              value={amount}
              onChange={(event) => onAmountChange(event.target.value)}
              className="rounded-md border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-900 shadow-sm focus:border-slate-500 focus:outline-none"
            />
          </div>
          <div className="grid gap-2">
            <label htmlFor="checkout-currency" className="text-sm font-semibold text-slate-700">Currency</label>
            <select
              id="checkout-currency"
              value={currency}
              onChange={(event) => onCurrencyChange(event.target.value)}
              className="rounded-md border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-900 shadow-sm focus:border-slate-500 focus:outline-none"
            >
              {currencyOptions.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </div>
        </div>

        <button
          type="submit"
          disabled={paymentLoading}
          className="mt-2 inline-flex items-center justify-center rounded-md bg-slate-900 px-4 py-3 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {paymentLoading ? "Processing..." : "Pay"}
        </button>
      </form>
    </section>
  );
}

export default ClientStripeCheckout;
