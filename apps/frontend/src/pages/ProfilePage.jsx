import { useEffect, useRef, useState } from "react";
import { Button } from "../components/Button";
import { Card } from "../components/Card";
import { api } from "../lib/api";
import { skinPreviewFor, skinThemeFor } from "../lib/skins";

function buildAvatarDataUrl(label, background, foreground = "#fffaf0") {
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96">
      <rect width="96" height="96" rx="32" fill="${background}" />
      <text x="50%" y="54%" text-anchor="middle" dominant-baseline="middle"
        font-family="Avenir Next, Segoe UI, sans-serif" font-size="40" font-weight="700" fill="${foreground}">
        ${label}
      </text>
    </svg>
  `;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

const avatarPresets = [
  { id: "ember", name: "Ember", url: buildAvatarDataUrl("S", "#d95d39") },
  { id: "mint", name: "Mint", url: buildAvatarDataUrl("9", "#5e6c4d") },
  { id: "midnight", name: "Midnight", url: buildAvatarDataUrl("C", "#161616") },
  { id: "sun", name: "Sun", url: buildAvatarDataUrl("A", "#d9a441", "#161616") },
  { id: "wave", name: "Wave", url: buildAvatarDataUrl("M", "#4d7c8a") },
  { id: "rose", name: "Rose", url: buildAvatarDataUrl("P", "#bb6b7a") }
];

const COUNTRY_SEARCH_API = "https://restcountries.com/v3.1/name";
const CITY_SEARCH_API = "https://geocoding-api.open-meteo.com/v1/search";
const skinDescriptions = {
  classic: "Warm tournament default with coral energy and clean contrast.",
  ember: "Hot ladder look with scorched reds and fast-match heat.",
  forest: "Calm green board for steady, low-tilt solving sessions.",
  midnight: "Deep navy premium look with sharp late-night focus.",
  solar: "Bright gold premium theme that feels high-rank and loud.",
  wave: "Cool cyan palette with airy board surfaces and smooth contrast."
};

function suggestionItems(options, value) {
  const query = value.trim().toLowerCase();
  const unique = Array.from(new Set(options.filter(Boolean)));
  if (!query) {
    return unique.slice(0, 6);
  }
  return unique.filter((item) => item.toLowerCase().includes(query)).slice(0, 6);
}

export function ProfilePage({ accessToken, user, equippedSkinId, onEquipSkin, onProfileUpdate, onLogout }) {
  const fileInputRef = useRef(null);
  const [form, setForm] = useState({
    avatarUrl: user.avatarUrl || "",
    countryCode: user.countryCode || "",
    city: user.city || ""
  });
  const [skins, setSkins] = useState([]);
  const [ownedSkins, setOwnedSkins] = useState([]);
  const [countries, setCountries] = useState([]);
  const [cities, setCities] = useState([]);
  const [remoteCountrySuggestions, setRemoteCountrySuggestions] = useState([]);
  const [remoteCitySuggestions, setRemoteCitySuggestions] = useState([]);
  const [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(true);
  const [checkoutSkinId, setCheckoutSkinId] = useState("");
  const [countryFocused, setCountryFocused] = useState(false);
  const [cityFocused, setCityFocused] = useState(false);
  const [selectedCountryAlpha2, setSelectedCountryAlpha2] = useState("");

  async function refreshOwnedSkins() {
    const mySkins = await api.mySkins(accessToken);
    const nextOwned = Array.isArray(mySkins) ? mySkins : [];
    setOwnedSkins(nextOwned);
    return nextOwned;
  }

  useEffect(() => {
    setForm({
      avatarUrl: user.avatarUrl || "",
      countryCode: user.countryCode || "",
      city: user.city || ""
    });
    setSelectedCountryAlpha2("");
  }, [user]);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      api.skins(accessToken),
      refreshOwnedSkins(),
      api.countries(accessToken),
      api.cities(accessToken)
    ])
      .then(([allSkins, mySkins, countryOptions, cityOptions]) => {
        setSkins(Array.isArray(allSkins) ? allSkins : []);
        setOwnedSkins(Array.isArray(mySkins) ? mySkins : []);
        setCountries(Array.isArray(countryOptions) ? countryOptions : []);
        setCities(Array.isArray(cityOptions) ? cityOptions : []);
      })
      .catch((error) => setNotice(error.message))
      .finally(() => setLoading(false));
  }, [accessToken]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const checkoutStatus = params.get("checkout");
    const sessionId = params.get("session_id");
    if (!checkoutStatus) {
      return;
    }

    let cancelled = false;

    async function syncCheckoutResult() {
      if (checkoutStatus === "cancelled") {
        if (!cancelled) {
          setNotice("Checkout cancelled.");
        }
        return;
      }

      if (checkoutStatus !== "success") {
        return;
      }

      if (sessionId) {
        try {
          const session = await api.checkoutSessionStatus(sessionId, accessToken);
          if (session?.paid) {
            await refreshOwnedSkins();
            if (!cancelled) {
              setNotice("Payment successful. Skin granted.");
            }
            return;
          }
        } catch {
          // Fall through to inventory polling below.
        }
      }

      for (let attempt = 0; attempt < 5; attempt += 1) {
        try {
          const nextOwned = await refreshOwnedSkins();
          if (nextOwned.length > 0) {
            if (!cancelled) {
              setNotice("Payment successful. Skin inventory updated.");
            }
            break;
          }
        } catch {
          // Retry briefly in case webhook delivery finishes after redirect.
        }
        await new Promise((resolve) => window.setTimeout(resolve, 900));
      }
    }

    void syncCheckoutResult().finally(() => {
      const next = new URL(window.location.href);
      next.searchParams.delete("checkout");
      next.searchParams.delete("session_id");
      window.history.replaceState({}, "", `${next.pathname}${next.search}`);
    });

    return () => {
      cancelled = true;
    };
  }, [accessToken]);

  useEffect(() => {
    const query = form.countryCode.trim();
    if (query.length < 2) {
      setRemoteCountrySuggestions([]);
      return undefined;
    }

    const controller = new AbortController();
    const timeoutId = window.setTimeout(async () => {
      try {
        const response = await fetch(
          `${COUNTRY_SEARCH_API}/${encodeURIComponent(query)}?fields=name,flags,cca2`,
          { signal: controller.signal }
        );
        if (!response.ok) {
          setRemoteCountrySuggestions([]);
          return;
        }
        const payload = await response.json();
        const nextSuggestions = Array.isArray(payload)
          ? payload
              .map((item) => ({
                code: item.cca2 || item.name?.common,
                name: item.name?.common || "",
                flag: item.flags?.svg || item.flags?.png || ""
              }))
              .filter((item) => item.name)
              .slice(0, 8)
          : [];
        setRemoteCountrySuggestions(nextSuggestions);
      } catch (error) {
        if (error.name !== "AbortError") {
          setRemoteCountrySuggestions([]);
        }
      }
    }, 220);

    return () => {
      controller.abort();
      window.clearTimeout(timeoutId);
    };
  }, [form.countryCode]);

  useEffect(() => {
    const query = form.city.trim();
    if (query.length < 2) {
      setRemoteCitySuggestions([]);
      return undefined;
    }

    const controller = new AbortController();
    const timeoutId = window.setTimeout(async () => {
      try {
        const url = new URL(CITY_SEARCH_API);
        url.searchParams.set("name", query);
        url.searchParams.set("count", "8");
        url.searchParams.set("language", "en");
        if (selectedCountryAlpha2) {
          url.searchParams.set("countryCode", selectedCountryAlpha2);
        }

        const response = await fetch(url.toString(), { signal: controller.signal });
        if (!response.ok) {
          setRemoteCitySuggestions([]);
          return;
        }
        const payload = await response.json();
        const nextSuggestions = Array.isArray(payload?.results)
          ? payload.results
              .map((item) => ({
                id: String(item.id || `${item.name}-${item.latitude}-${item.longitude}`),
                name: item.name || "",
                country: item.country || "",
                admin1: item.admin1 || "",
                countryCode: item.country_code || ""
              }))
              .filter((item) => item.name)
              .slice(0, 8)
          : [];
        setRemoteCitySuggestions(nextSuggestions);
      } catch (error) {
        if (error.name !== "AbortError") {
          setRemoteCitySuggestions([]);
        }
      }
    }, 220);

    return () => {
      controller.abort();
      window.clearTimeout(timeoutId);
    };
  }, [form.city, selectedCountryAlpha2]);

  async function saveProfile(event) {
    event.preventDefault();
    try {
      const updated = await api.updateMe(form, accessToken);
      onProfileUpdate(updated);
      setNotice("Profile updated.");
    } catch (error) {
      setNotice(error.message);
    }
  }

  async function unlockSkin(skinId) {
    try {
      await api.purchaseSkin(skinId, accessToken);
      const nextOwned = await refreshOwnedSkins();
      setOwnedSkins(nextOwned);
      const unlocked = nextOwned.find((item) => item.id === skinId);
      if ((!equippedSkinId || equippedSkinId === "classic") && unlocked) {
        onEquipSkin?.(String(unlocked.name || "classic").toLowerCase());
      }
      setNotice("Skin added to inventory.");
    } catch (error) {
      setNotice(error.message);
    }
  }

  async function checkoutSkin(skinId) {
    setCheckoutSkinId(skinId);
    try {
      const session = await api.createCheckoutSession(skinId, accessToken);
      if (session?.url) {
        window.location.href = session.url;
        return;
      }
      setNotice("Checkout session created, but no redirect URL was returned.");
    } catch (error) {
      setNotice(error.message);
    } finally {
      setCheckoutSkinId("");
    }
  }

  async function equipSkin(skinKey) {
    try {
      const updated = await api.updateMe({ activeSkin: skinKey }, accessToken);
      onEquipSkin?.(skinKey);
      onProfileUpdate(updated);
      setNotice("Skin equipped.");
    } catch (error) {
      setNotice(error.message);
    }
  }

  function openFilePicker() {
    fileInputRef.current?.click();
  }

  function handleAvatarFileChange(event) {
    const [file] = event.target.files || [];
    if (!file) {
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") {
        setForm((current) => ({ ...current, avatarUrl: reader.result }));
      }
    };
    reader.readAsDataURL(file);
  }

  const countryOptions = Array.from(new Set([form.countryCode, ...countries].filter(Boolean)));
  const cityOptions = Array.from(new Set([form.city, ...cities].filter(Boolean)));
  const localCountrySuggestions = suggestionItems(countryOptions, form.countryCode);
  const localCitySuggestions = suggestionItems(cityOptions, form.city).map((city) => ({
    id: city,
    name: city,
    country: form.countryCode || "",
    admin1: "",
    countryCode: selectedCountryAlpha2
  }));
  const countrySuggestions =
    remoteCountrySuggestions.length > 0
      ? remoteCountrySuggestions
      : localCountrySuggestions.map((country) => ({
          code: country,
          name: country,
          flag: ""
        }));
  const citySuggestions = remoteCitySuggestions.length > 0 ? remoteCitySuggestions : localCitySuggestions;

  return (
    <div className="space-y-4">
      <Card title="Me" subtitle="Your live account snapshot.">
        <div className="grid grid-cols-3 gap-3">
          <div className="rounded-3xl bg-white/75 px-3 py-4 text-center">
            <div className="text-xl font-black">{user.eloRating}</div>
            <div className="mt-1 text-xs uppercase tracking-[0.2em] text-slatewarm">ELO</div>
          </div>
          <div className="rounded-3xl bg-white/75 px-3 py-4 text-center">
            <div className="text-xl font-black">{user.wins}</div>
            <div className="mt-1 text-xs uppercase tracking-[0.2em] text-slatewarm">Wins</div>
          </div>
          <div className="rounded-3xl bg-white/75 px-3 py-4 text-center">
            <div className="text-xl font-black">{user.level}</div>
            <div className="mt-1 text-xs uppercase tracking-[0.2em] text-slatewarm">Level</div>
          </div>
        </div>
      </Card>

      <Card title={user.username || "Me"} subtitle="Tune your identity for the mobile arena.">
        <form className="space-y-4" onSubmit={saveProfile}>
          <div className="space-y-3">
            <div className="flex items-center gap-4 rounded-[2rem] bg-white/75 px-4 py-4">
              <img
                alt="Avatar preview"
                className="h-20 w-20 rounded-[1.75rem] object-cover shadow-card"
                src={form.avatarUrl || avatarPresets[0].url}
              />
              <div>
                <div className="text-sm uppercase tracking-[0.22em] text-slatewarm">Avatar</div>
                <div className="mt-1 text-lg font-black">Pick your arena face</div>
                <div className="mt-1 text-sm text-slatewarm">Choose a photo from your device or keep a quick preset.</div>
              </div>
            </div>

            <input
              ref={fileInputRef}
              accept="image/*"
              className="hidden"
              onChange={handleAvatarFileChange}
              type="file"
            />
            <Button className="w-full" onClick={openFilePicker} type="button" variant="soft">
              Choose Photo From Files
            </Button>

            <div className="grid grid-cols-3 gap-3">
              {avatarPresets.map((avatar) => {
                const selected = form.avatarUrl === avatar.url;
                return (
                  <button
                    key={avatar.id}
                    className={`rounded-[1.75rem] border px-3 py-3 text-center transition ${
                      selected ? "border-ink bg-ink text-sand" : "border-white/80 bg-white/80 text-ink"
                    }`}
                    onClick={() => setForm((current) => ({ ...current, avatarUrl: avatar.url }))}
                    type="button"
                  >
                    <img alt={avatar.name} className="mx-auto h-14 w-14 rounded-2xl object-cover" src={avatar.url} />
                    <div className="mt-2 text-xs font-bold">{avatar.name}</div>
                  </button>
                );
              })}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="relative">
              <span className="mb-2 block text-sm font-semibold text-slatewarm">Country</span>
              <input
                autoComplete="country-name"
                className="w-full rounded-3xl border border-white bg-white/80 px-4 py-3 text-base outline-none placeholder:text-slatewarm/70 focus:border-ink/20"
                placeholder="Start typing a country"
                type="text"
                value={form.countryCode}
                onBlur={() => window.setTimeout(() => setCountryFocused(false), 120)}
                onChange={(event) => {
                  setSelectedCountryAlpha2("");
                  setForm((current) => ({ ...current, countryCode: event.target.value }));
                }}
                onFocus={() => setCountryFocused(true)}
              />
              {countryFocused && countrySuggestions.length > 0 ? (
                <div className="absolute left-0 right-0 top-[calc(100%+0.5rem)] z-20 rounded-[1.5rem] border border-white/80 bg-[#fffaf0] p-2 shadow-card">
                  {countrySuggestions.map((country) => (
                    <button
                      key={country.code}
                      className="flex w-full items-center gap-3 rounded-2xl px-3 py-2 text-left text-sm font-semibold text-ink transition hover:bg-ink hover:text-sand"
                      onMouseDown={(event) => {
                        event.preventDefault();
                        setSelectedCountryAlpha2(country.code || "");
                        setForm((current) => ({ ...current, countryCode: country.name }));
                        setCountryFocused(false);
                      }}
                      type="button"
                    >
                      {country.flag ? (
                        <img alt="" className="h-5 w-7 rounded-md object-cover" src={country.flag} />
                      ) : (
                        <div className="h-5 w-7 rounded-md bg-ink/10" />
                      )}
                      <span>{country.name}</span>
                    </button>
                  ))}
                </div>
              ) : null}
            </div>

            <div className="relative">
              <span className="mb-2 block text-sm font-semibold text-slatewarm">City</span>
              <input
                autoComplete="address-level2"
                className="w-full rounded-3xl border border-white bg-white/80 px-4 py-3 text-base outline-none placeholder:text-slatewarm/70 focus:border-ink/20"
                placeholder="Start typing a city"
                type="text"
                value={form.city}
                onBlur={() => window.setTimeout(() => setCityFocused(false), 120)}
                onChange={(event) => setForm((current) => ({ ...current, city: event.target.value }))}
                onFocus={() => setCityFocused(true)}
              />
              {cityFocused && citySuggestions.length > 0 ? (
                <div className="absolute left-0 right-0 top-[calc(100%+0.5rem)] z-20 rounded-[1.5rem] border border-white/80 bg-[#fffaf0] p-2 shadow-card">
                  {citySuggestions.map((city) => (
                    <button
                      key={city.id}
                      className="block w-full rounded-2xl px-3 py-2 text-left text-sm font-semibold text-ink transition hover:bg-ink hover:text-sand"
                      onMouseDown={(event) => {
                        event.preventDefault();
                        setForm((current) => ({ ...current, city: city.name }));
                        setCityFocused(false);
                      }}
                      type="button"
                    >
                      <div>{city.name}</div>
                      {city.admin1 || city.country ? (
                        <div className="text-xs font-medium text-slatewarm">
                          {[city.admin1, city.country].filter(Boolean).join(", ")}
                        </div>
                      ) : null}
                    </button>
                  ))}
                </div>
              ) : null}
            </div>
          </div>
          <div className="flex gap-3">
            <Button className="flex-1" type="submit">Save</Button>
            <Button className="flex-1" type="button" variant="ghost" onClick={onLogout}>Logout</Button>
          </div>
        </form>
        {notice ? <p className="mt-3 text-sm text-slatewarm">{notice}</p> : null}
      </Card>

      <Card title="Cosmetics" subtitle="Unlock skins from the same API the backend exposes.">
        {loading ? (
          <div className="rounded-3xl bg-white/75 px-4 py-4 text-sm text-slatewarm">Loading cosmetics...</div>
        ) : (
          <div className="space-y-3">
          {skins.map((skin) => {
            const owned = ownedSkins.some((item) => item.id === skin.id);
            const skinKey = String(skin.name || "classic").toLowerCase();
            const equipped = equippedSkinId === skinKey;
            const theme = skinThemeFor(skin);
            const priceLabel = skin.priceUsd > 0 ? `$${skin.priceUsd}` : "Free";
            const description = skinDescriptions[skinKey] || "Arena theme for boards, shell and match progress.";
            return (
              <div key={skin.id} className="overflow-hidden rounded-[2rem] border border-white/80 bg-white/75 shadow-card">
                <div className="px-4 pt-4">
                  <div className={`overflow-hidden rounded-[1.75rem] border ${theme.boardFrame}`}>
                    <div className={`flex items-center justify-between px-4 py-3 ${theme.shellAccent}`}>
                      <div>
                        <div className="text-[10px] uppercase tracking-[0.26em] opacity-80">Sudoku Clash</div>
                        <div className="mt-1 text-base font-black">{skin.name}</div>
                      </div>
                      <div className={`rounded-full px-3 py-2 text-[11px] font-bold shadow-sm ${theme.navActive}`}>
                        1124 ELO
                      </div>
                    </div>
                    <div className="grid grid-cols-[1.35fr,0.95fr] gap-3 p-3">
                      <div className="grid grid-cols-3 gap-[3px] rounded-[1.1rem] bg-black/10 p-[3px]">
                        {[0, 1, 2, 3, 4, 5, 6, 7, 8].map((cell) => {
                          const editable = [1, 3, 4, 7].includes(cell);
                          const active = cell === 4;
                          return (
                            <div
                              key={cell}
                              className={`flex aspect-square items-center justify-center rounded-[0.6rem] text-[11px] font-black ${
                                editable ? theme.boardEditable : theme.boardFixed
                              } ${active ? "ring-2 ring-black/20" : ""}`}
                            >
                              {editable ? (cell % 2 === 0 ? "" : 7) : ((cell + 2) % 9) + 1}
                            </div>
                          );
                        })}
                      </div>
                      <div className="flex flex-col justify-between gap-2">
                        <div className="rounded-[1.15rem] bg-white/65 px-3 py-3">
                          <div className="text-[10px] uppercase tracking-[0.2em] text-slatewarm">Progress</div>
                          <div className="mt-1 text-lg font-black text-ink">19 / 27</div>
                        </div>
                        <div className="rounded-[1.15rem] bg-white/65 px-3 py-3">
                          <div className="text-[10px] uppercase tracking-[0.2em] text-slatewarm">Theme</div>
                          <div className="mt-1 flex gap-2">
                            <div className={`h-3 w-3 rounded-full ${theme.shellAccent.split(" ").find((item) => item.startsWith("bg-")) || "bg-coral"}`} />
                            <div className={`h-3 w-3 rounded-full ${theme.navActive.split(" ").find((item) => item.startsWith("bg-")) || "bg-ink"}`} />
                            <div className={`h-3 w-3 rounded-full ${theme.boardFixed.split(" ").find((item) => item.startsWith("bg-")) || "bg-sand"}`} />
                            <div className={`h-3 w-3 rounded-full ${theme.boardEditable.split(" ").find((item) => item.startsWith("bg-")) || "bg-white"}`} />
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div className="flex items-start gap-4 px-4 py-4">
                  <img alt={skin.name} className="h-16 w-16 rounded-[1.25rem] object-cover shadow-card" src={skinPreviewFor(skin)} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <div className="font-black">{skin.name}</div>
                      {equipped ? <span className={`rounded-full px-2 py-1 text-[10px] font-bold ${theme.shellAccent}`}>Equipped</span> : null}
                      {skin.isPremium ? <span className="rounded-full bg-ink px-2 py-1 text-[10px] font-bold text-sand">Premium</span> : null}
                    </div>
                    <div className="mt-1 text-xs text-slatewarm">
                      {priceLabel} · {skin.isPremium ? "premium skin" : "standard skin"}
                    </div>
                    <div className="mt-2 text-sm leading-6 text-ink/80">
                      {description}
                    </div>
                  </div>
                  {owned ? (
                    <Button
                      variant={equipped ? "soft" : "accent"}
                      disabled={equipped}
                      onClick={() => equipSkin(skinKey)}
                    >
                      {equipped ? "Equipped" : "Equip"}
                    </Button>
                  ) : (
                    <Button
                      loading={checkoutSkinId === skin.id}
                      variant="accent"
                      onClick={() => (skin.priceUsd > 0 ? checkoutSkin(skin.id) : unlockSkin(skin.id))}
                    >
                      {skin.priceUsd > 0 ? `Buy $${skin.priceUsd}` : "Unlock"}
                    </Button>
                  )}
                </div>
              </div>
            );
          })}
          </div>
        )}
      </Card>
    </div>
  );
}
