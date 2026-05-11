function makePreview(label, background, foreground = "#fffaf0") {
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 120">
      <defs>
        <linearGradient id="g" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stop-color="${background}" stop-opacity="1" />
          <stop offset="100%" stop-color="${foreground}" stop-opacity="0.22" />
        </linearGradient>
      </defs>
      <rect width="240" height="120" rx="34" fill="url(#g)" />
      <rect x="18" y="18" width="72" height="72" rx="22" fill="rgba(255,250,240,0.24)" />
      <text x="54" y="62" text-anchor="middle" dominant-baseline="middle"
        font-family="Avenir Next, Segoe UI, sans-serif" font-size="28" font-weight="700" fill="#fffaf0">
        ${label}
      </text>
      <text x="114" y="55" font-family="Avenir Next, Segoe UI, sans-serif" font-size="22" font-weight="700" fill="#fffaf0">
        Skin
      </text>
      <text x="114" y="80" font-family="Avenir Next, Segoe UI, sans-serif" font-size="12" fill="#fffaf0">
        Mobile Arena Theme
      </text>
    </svg>
  `;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

const catalog = {
  classic: {
    id: "classic",
    name: "Classic",
    previewUrl: makePreview("C", "#d95d39"),
    shellAccent: "bg-coral text-white",
    navActive: "bg-ink text-sand",
    boardFrame: "border-ink bg-ink",
    boardFixed: "bg-wheat text-ink",
    boardEditable: "bg-white text-coral"
  },
  ember: {
    id: "ember",
    name: "Ember",
    previewUrl: makePreview("E", "#c9582f"),
    shellAccent: "bg-[#c9582f] text-white",
    navActive: "bg-[#2f130f] text-[#fff3e6]",
    boardFrame: "border-[#2f130f] bg-[#2f130f]",
    boardFixed: "bg-[#f1d0bc] text-[#2f130f]",
    boardEditable: "bg-[#fff7f1] text-[#c9582f]"
  },
  forest: {
    id: "forest",
    name: "Forest",
    previewUrl: makePreview("F", "#5e6c4d"),
    shellAccent: "bg-[#5e6c4d] text-white",
    navActive: "bg-[#203123] text-[#eef5e8]",
    boardFrame: "border-[#203123] bg-[#203123]",
    boardFixed: "bg-[#d8e2cf] text-[#203123]",
    boardEditable: "bg-[#f5f9f0] text-[#5e6c4d]"
  },
  midnight: {
    id: "midnight",
    name: "Midnight",
    previewUrl: makePreview("M", "#2d3f72"),
    shellAccent: "bg-[#2d3f72] text-white",
    navActive: "bg-[#101a33] text-[#eef3ff]",
    boardFrame: "border-[#101a33] bg-[#101a33]",
    boardFixed: "bg-[#d4ddf5] text-[#101a33]",
    boardEditable: "bg-[#f4f7ff] text-[#2d3f72]"
  },
  solar: {
    id: "solar",
    name: "Solar",
    previewUrl: makePreview("S", "#d9a441", "#161616"),
    shellAccent: "bg-[#d9a441] text-[#161616]",
    navActive: "bg-[#6f4c00] text-[#fff4d6]",
    boardFrame: "border-[#6f4c00] bg-[#6f4c00]",
    boardFixed: "bg-[#f6e1aa] text-[#6f4c00]",
    boardEditable: "bg-[#fffaf0] text-[#b57b00]"
  },
  wave: {
    id: "wave",
    name: "Wave",
    previewUrl: makePreview("W", "#4d7c8a"),
    shellAccent: "bg-[#4d7c8a] text-white",
    navActive: "bg-[#17313a] text-[#ecfbff]",
    boardFrame: "border-[#17313a] bg-[#17313a]",
    boardFixed: "bg-[#cce1e8] text-[#17313a]",
    boardEditable: "bg-[#f4fcff] text-[#4d7c8a]"
  }
};

export function skinThemeFor(skin) {
  if (!skin) {
    return catalog.classic;
  }
  const rawKey =
    typeof skin === "string"
      ? skin
      : skin.name || skin.code || skin.slug || skin.activeSkin || skin.id || "classic";
  const key = String(rawKey).toLowerCase();
  return catalog[key] || catalog.classic;
}

export function skinPreviewFor(skin) {
  const theme = skinThemeFor(skin);
  return skin?.previewUrl || theme.previewUrl;
}
