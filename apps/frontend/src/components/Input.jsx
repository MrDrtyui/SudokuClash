export function Input({ label, className = "", ...props }) {
  return (
    <label className="block">
      <span className="mb-2 block text-sm font-semibold text-slatewarm">{label}</span>
      <input
        className={`w-full rounded-3xl border border-white bg-white/80 px-4 py-3 text-base outline-none ring-0 placeholder:text-slatewarm/70 focus:border-ink/20 ${className}`}
        {...props}
      />
    </label>
  );
}

