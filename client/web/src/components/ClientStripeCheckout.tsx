import { FormEvent, useEffect, useMemo, useState } from "react";

type ClientStripeCheckoutProps = {
  amount: string;
  currency: string;
  paymentLoading: boolean;
  onAmountChange: (value: string) => void;
  onCurrencyChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onSimulateOutcomeChange: (value: "SUCCESS" | "FAILED") => void;
};

function formatCard(value: string): string {
  const digits = value.replace(/\D/g, "").slice(0, 16);
  return digits.replace(/(.{4})/g, "$1 ").trim();
}

function ClientStripeCheckout({
  amount,
  currency,
  paymentLoading,
  onAmountChange,
  onCurrencyChange,
  onSubmit,
  onSimulateOutcomeChange,
}: ClientStripeCheckoutProps) {
  const [email, setEmail] = useState("customer@example.com");
  const [cardName, setCardName] = useState("John Doe");
  const [cardNumber, setCardNumber] = useState("4242 4242 4242 4242");
  const [expiry, setExpiry] = useState("12/34");
  const [cvc, setCvc] = useState("123");

  const normalizedCard = useMemo(() => cardNumber.replace(/\s/g, ""), [cardNumber]);

  useEffect(() => {
    // Stripe-like test behavior: card ending 0002 fails.
    onSimulateOutcomeChange(normalizedCard.endsWith("0002") ? "FAILED" : "SUCCESS");
  }, [normalizedCard, onSimulateOutcomeChange]);

  return (
    <section className="h-full rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="mb-4">
        <h2 className="text-lg font-semibold text-slate-900">Stripe Checkout (Client View)</h2>
        <p className="mt-1 text-sm text-slate-600">Enter payment details and submit to the selected distributed node.</p>
      </div>

      <form className="grid gap-4" onSubmit={onSubmit}>
        <div className="grid gap-2">
          <label htmlFor="checkout-email" className="text-sm font-medium text-slate-700">Email</label>
          <input
            id="checkout-email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            className="rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
            placeholder="you@example.com"
          />
        </div>

        <div className="grid gap-2">
          <label htmlFor="checkout-name" className="text-sm font-medium text-slate-700">Cardholder Name</label>
          <input
            id="checkout-name"
            value={cardName}
            onChange={(event) => setCardName(event.target.value)}
            className="rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
            placeholder="Name on card"
          />
        </div>

        <div className="grid gap-2">
          <label htmlFor="checkout-card" className="text-sm font-medium text-slate-700">Card Number</label>
          <input
            id="checkout-card"
            value={cardNumber}
            onChange={(event) => setCardNumber(formatCard(event.target.value))}
            className="rounded-md border border-slate-300 px-3 py-2 font-mono text-sm focus:border-slate-400 focus:outline-none"
            placeholder="4242 4242 4242 4242"
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="grid gap-2">
            <label htmlFor="checkout-expiry" className="text-sm font-medium text-slate-700">Expiry</label>
            <input
              id="checkout-expiry"
              value={expiry}
              onChange={(event) => setExpiry(event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
              placeholder="MM/YY"
            />
          </div>
          <div className="grid gap-2">
            <label htmlFor="checkout-cvc" className="text-sm font-medium text-slate-700">CVC</label>
            <input
              id="checkout-cvc"
              value={cvc}
              onChange={(event) => setCvc(event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none"
              placeholder="123"
            />
          </div>
        </div>

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
            <input
              id="checkout-currency"
              value={currency}
              onChange={(event) => onCurrencyChange(event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm uppercase focus:border-slate-400 focus:outline-none"
              placeholder="USD"
            />
          </div>
        </div>

        <div className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600">
          Test cards: 4242 ... 4242 = success, 4000 ... 0002 = failed simulation.
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
