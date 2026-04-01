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
    <section className="h-full rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="mb-4">
        <h2 className="text-lg font-semibold text-slate-900">Stripe Checkout (Client View)</h2>
        <p className="mt-1 text-sm text-slate-600">Enter payment amount and currency. Stripe checkout will collect card details securely.</p>
      </div>

      <form className="grid gap-4" onSubmit={onSubmit}>
        <div className="grid grid-cols-2 gap-3">
          <div className="grid gap-2">
            <label htmlFor="checkout-amount" className="text-sm font-medium text-slate-700">Amount</label>
            <input
              id="checkout-amount"
              type="number"
              min="0"
              step="0.01"
              value={amount}
              onChange={(event) => onAmountChange(event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
            />
          </div>
          <div className="grid gap-2">
            <label htmlFor="checkout-currency" className="text-sm font-medium text-slate-700">Currency</label>
            <select
              id="checkout-currency"
              value={currency}
              onChange={(event) => onCurrencyChange(event.target.value)}
              className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
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
          className="mt-1 rounded-md bg-slate-900 px-4 py-2.5 text-sm font-medium text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {paymentLoading ? "Processing..." : "Pay"}
        </button>
      </form>
    </section>
  );
}

export default ClientStripeCheckout;
