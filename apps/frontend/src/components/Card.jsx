export function Card({ title, subtitle, right, children, className = "" }) {
  return (
    <section className={`glass rounded-4xl border border-white/80 p-5 shadow-card ${className}`}>
      {(title || right) && (
        <div className="mb-4 flex items-start justify-between gap-3">
          <div>
            {title ? <h2 className="text-lg font-black">{title}</h2> : null}
            {subtitle ? <p className="mt-1 text-sm text-slatewarm">{subtitle}</p> : null}
          </div>
          {right}
        </div>
      )}
      {children}
    </section>
  );
}

