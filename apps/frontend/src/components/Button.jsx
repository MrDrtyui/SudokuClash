export function Button({
  children,
  className = "",
  variant = "primary",
  loading = false,
  ...props
}) {
  const styles = {
    primary: "bg-ink text-sand",
    accent: "bg-coral text-white",
    soft: "bg-white/80 text-ink",
    ghost: "bg-transparent text-ink border border-ink/10"
  };

  return (
    <button
      className={`rounded-full px-4 py-3 text-sm font-bold transition active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50 ${styles[variant]} ${className}`}
      disabled={loading || props.disabled}
      {...props}
    >
      {loading ? "Loading..." : children}
    </button>
  );
}

