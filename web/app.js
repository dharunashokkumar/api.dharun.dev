(() => {
  const DATA_URL = "data/logos.json";
  const PAGE_SIZE = 72;
  const EXPORT_SIZE = 512;
  const MAX_RASTER_WITHOUT_CONFIRM = 300;

  const els = {
    statTotal: document.querySelector("#statTotal"),
    statNse: document.querySelector("#statNse"),
    statBse: document.querySelector("#statBse"),
    searchInput: document.querySelector("#searchInput"),
    clearSearch: document.querySelector("#clearSearch"),
    exchangeFilter: document.querySelector("#exchangeFilter"),
    sectorFilter: document.querySelector("#sectorFilter"),
    indexFilter: document.querySelector("#indexFilter"),
    sortSelect: document.querySelector("#sortSelect"),
    scaleSelect: document.querySelector("#scaleSelect"),
    qualityInput: document.querySelector("#qualityInput"),
    qualityValue: document.querySelector("#qualityValue"),
    downloadSelected: document.querySelector("#downloadSelected"),
    downloadResults: document.querySelector("#downloadResults"),
    resultCount: document.querySelector("#resultCount"),
    selectedCount: document.querySelector("#selectedCount"),
    selectVisible: document.querySelector("#selectVisible"),
    clearSelected: document.querySelector("#clearSelected"),
    results: document.querySelector("#results"),
    loadMore: document.querySelector("#loadMore"),
    toast: document.querySelector("#toast"),
    template: document.querySelector("#logoCardTemplate"),
    apiForm: document.querySelector("#apiForm"),
    apiTickerInput: document.querySelector("#apiTickerInput"),
    apiExchangeSelect: document.querySelector("#apiExchangeSelect"),
    apiLogoPreview: document.querySelector("#apiLogoPreview"),
    apiMatchExchange: document.querySelector("#apiMatchExchange"),
    apiMatchTicker: document.querySelector("#apiMatchTicker"),
    apiMatchCompany: document.querySelector("#apiMatchCompany"),
    apiMatchMeta: document.querySelector("#apiMatchMeta"),
    apiSvgUrl: document.querySelector("#apiSvgUrl"),
    apiHtmlSnippet: document.querySelector("#apiHtmlSnippet"),
    apiJsonUrl: document.querySelector("#apiJsonUrl"),
    apiStatus: document.querySelector("#apiStatus"),
    apiCopyButtons: document.querySelectorAll("[data-copy-target]"),
  };

  const collator = new Intl.Collator("en", { numeric: true, sensitivity: "base" });
  const numberFormatter = new Intl.NumberFormat("en-IN", {
    maximumFractionDigits: 1,
    notation: "compact",
  });

  const state = {
    logos: [],
    filtered: [],
    selected: new Set(),
    visible: PAGE_SIZE,
    search: "",
    exchange: "all",
    sector: "all",
    index: "all",
    sort: "relevance",
    format: "svg",
    scale: 2,
    quality: 0.92,
    apiTicker: "",
    apiExchange: "auto",
    busy: false,
  };

  init();

  async function init() {
    bindControls();
    readUrlState();

    try {
      const response = await fetch(DATA_URL);
      if (!response.ok) {
        throw new Error(`Could not load ${DATA_URL}`);
      }
      const data = await response.json();
      state.logos = data.logos.map(prepareLogo);
      hydrateCounts(data.counts);
      hydrateFilters(data);
      syncControls();
      syncApiControls();
      resolveApiLogo();
      applyFilters();
    } catch (error) {
      console.error(error);
      if (els.apiStatus) {
        els.apiStatus.textContent = "The logo index could not be loaded.";
      }
      showEmpty("The logo index could not be loaded.");
    }
  }

  function bindControls() {
    const debouncedSearch = debounce(() => applyFilters(), 180);

    els.searchInput.addEventListener("input", () => {
      state.search = els.searchInput.value.trim();
      state.visible = PAGE_SIZE;
      debouncedSearch();
    });

    els.clearSearch.addEventListener("click", () => {
      state.search = "";
      els.searchInput.value = "";
      state.visible = PAGE_SIZE;
      applyFilters();
      els.searchInput.focus();
    });

    for (const [element, key] of [
      [els.exchangeFilter, "exchange"],
      [els.sectorFilter, "sector"],
      [els.indexFilter, "index"],
      [els.sortSelect, "sort"],
    ]) {
      element.addEventListener("change", () => {
        state[key] = element.value;
        state.visible = PAGE_SIZE;
        applyFilters();
      });
    }

    document.querySelectorAll("input[name='format']").forEach((input) => {
      input.addEventListener("change", () => {
        state.format = input.value;
      });
    });

    els.scaleSelect.addEventListener("change", () => {
      state.scale = Number(els.scaleSelect.value);
    });

    els.qualityInput.addEventListener("input", () => {
      state.quality = Number(els.qualityInput.value) / 100;
      els.qualityValue.textContent = els.qualityInput.value;
    });

    els.loadMore.addEventListener("click", () => {
      state.visible += PAGE_SIZE;
      render();
    });

    els.selectVisible.addEventListener("click", () => {
      state.filtered.slice(0, state.visible).forEach((logo) => state.selected.add(logo.id));
      render();
    });

    els.clearSelected.addEventListener("click", () => {
      state.selected.clear();
      render();
    });

    els.downloadSelected.addEventListener("click", () => {
      const logos = state.logos.filter((logo) => state.selected.has(logo.id));
      downloadBatch(logos, "selected");
    });

    els.downloadResults.addEventListener("click", () => {
      downloadBatch(state.filtered, "results");
    });

    if (els.apiForm) {
      const debouncedApiPreview = debounce(() => resolveApiLogo(), 160);

      els.apiForm.addEventListener("submit", (event) => {
        event.preventDefault();
        state.apiTicker = els.apiTickerInput.value.trim();
        state.apiExchange = els.apiExchangeSelect.value;
        resolveApiLogo({ syncSearch: true, updateAddress: true });
      });

      els.apiTickerInput.addEventListener("input", () => {
        state.apiTicker = els.apiTickerInput.value.trim();
        debouncedApiPreview();
      });

      els.apiExchangeSelect.addEventListener("change", () => {
        state.apiExchange = els.apiExchangeSelect.value;
        resolveApiLogo();
      });

      els.apiCopyButtons.forEach((button) => {
        button.addEventListener("click", () => {
          copyTarget(button.dataset.copyTarget);
        });
      });
    }
  }

  function readUrlState() {
    const params = new URLSearchParams(window.location.search);
    state.search = params.get("q") || "";
    state.exchange = params.get("exchange") || "all";
    state.sector = params.get("sector") || "all";
    state.index = params.get("index") || "all";
    state.sort = params.get("sort") || "relevance";
    state.apiTicker = params.get("ticker") || params.get("symbol") || "";

    const parsedApi = parseTickerInput(state.apiTicker);
    const apiExchange = (params.get("apiExchange") || params.get("market") || "").toUpperCase();
    if (apiExchange === "NSE" || apiExchange === "BSE") {
      state.apiExchange = apiExchange;
    }
    if (parsedApi.exchange) {
      state.apiExchange = parsedApi.exchange;
    }
    if (!state.search && parsedApi.ticker) {
      state.search = parsedApi.ticker;
    }
  }

  function syncControls() {
    els.searchInput.value = state.search;
    els.exchangeFilter.value = state.exchange;
    els.sectorFilter.value = state.sector;
    els.indexFilter.value = state.index;
    els.sortSelect.value = state.sort;
    els.scaleSelect.value = String(state.scale);
    els.qualityInput.value = String(Math.round(state.quality * 100));
    els.qualityValue.textContent = els.qualityInput.value;
    document.querySelector(`input[name='format'][value='${state.format}']`).checked = true;
  }

  function syncApiControls() {
    if (!els.apiForm) return;
    const parsedApi = parseTickerInput(state.apiTicker);
    els.apiTickerInput.value = parsedApi.ticker || state.apiTicker;
    els.apiExchangeSelect.value = state.apiExchange;
    refreshApiCopyButtons();
  }

  function hydrateCounts(counts) {
    if (!counts) return;
    els.statTotal.textContent = counts.total_logos ?? state.logos.length;
    els.statNse.textContent = counts.by_exchange?.NSE ?? "";
    els.statBse.textContent = counts.by_exchange?.BSE ?? "";
  }

  function hydrateFilters(data) {
    for (const sector of data.sectors || []) {
      const option = document.createElement("option");
      option.value = sector;
      option.textContent = sector;
      els.sectorFilter.append(option);
    }

    for (const indexName of data.indexes || []) {
      const option = document.createElement("option");
      option.value = indexName;
      option.textContent = indexName;
      els.indexFilter.append(option);
    }
  }

  function prepareLogo(logo) {
    const searchParts = [
      logo.id,
      logo.exchange,
      logo.ticker,
      logo.company,
      logo.sector,
      logo.industry,
      logo.isin,
      ...(logo.indexes || []),
    ];

    return {
      ...logo,
      searchText: normalize(searchParts.filter(Boolean).join(" ")),
      marketCapValue: Number(logo.marketCap || 0),
    };
  }

  function resolveApiLogo(options = {}) {
    if (!els.apiForm) return;

    const { syncSearch = false, updateAddress = false } = options;
    const parsed = parseTickerInput(state.apiTicker);
    const exchange = parsed.exchange || state.apiExchange;

    if (parsed.exchange) {
      state.apiExchange = parsed.exchange;
      els.apiExchangeSelect.value = parsed.exchange;
    }

    if (!parsed.ticker) {
      renderApiPlaceholder("Enter a symbol to generate a public logo URL.", "Waiting for a ticker.");
      if (updateAddress) updateUrl();
      return;
    }

    const logo = findLogoByTicker(parsed.ticker, exchange);
    if (!logo) {
      renderApiPlaceholder(
        `No logo found for ${parsed.ticker}${exchange !== "auto" ? ` on ${exchange}` : ""}.`,
        "No matching logo endpoint.",
      );
      if (syncSearch) {
        state.search = parsed.ticker;
        state.exchange = exchange === "auto" ? "all" : exchange;
        state.visible = PAGE_SIZE;
        syncControls();
        applyFilters();
      } else if (updateAddress) {
        updateUrl();
      }
      return;
    }

    renderApiMatch(logo);

    if (syncSearch) {
      state.search = logo.ticker;
      state.exchange = exchange === "auto" ? "all" : logo.exchange;
      state.visible = PAGE_SIZE;
      syncControls();
      applyFilters();
    } else if (updateAddress) {
      updateUrl();
    }
  }

  function renderApiMatch(logo) {
    const svgUrl = apiUrl("logo", logo);
    const company = logo.company || logo.ticker;

    els.apiLogoPreview.src = logo.file;
    els.apiLogoPreview.alt = `${company} logo`;
    els.apiMatchExchange.textContent = logo.exchange;
    els.apiMatchTicker.textContent = logo.ticker;
    els.apiMatchCompany.textContent = company;
    els.apiMatchMeta.textContent = `${logo.sector || "Listed company"} - ${logo.id}`;
    els.apiSvgUrl.value = svgUrl;
    els.apiHtmlSnippet.value = `<img src="${svgUrl}" alt="${escapeAttribute(company)} logo" loading="lazy">`;
    els.apiJsonUrl.value = apiUrl("companies", logo);
    els.apiStatus.textContent = `Ready: ${logo.id}`;
    refreshApiCopyButtons();
  }

  function renderApiPlaceholder(message, status) {
    els.apiLogoPreview.removeAttribute("src");
    els.apiLogoPreview.alt = "";
    els.apiMatchExchange.textContent = "API";
    els.apiMatchTicker.textContent = "Ticker";
    els.apiMatchCompany.textContent = message;
    els.apiMatchMeta.textContent = "Logo endpoint";
    els.apiSvgUrl.value = "";
    els.apiHtmlSnippet.value = "";
    els.apiJsonUrl.value = "";
    els.apiStatus.textContent = status;
    refreshApiCopyButtons();
  }

  function findLogoByTicker(ticker, exchange) {
    const normalizedTicker = normalizeTicker(ticker);
    const candidates = state.logos.filter((logo) => {
      if (normalizeTicker(logo.ticker) !== normalizedTicker) return false;
      return exchange === "auto" || logo.exchange === exchange;
    });

    return candidates.sort((a, b) => {
      if (a.exchange === "NSE" && b.exchange !== "NSE") return -1;
      if (b.exchange === "NSE" && a.exchange !== "NSE") return 1;
      return b.marketCapValue - a.marketCapValue || collator.compare(a.ticker, b.ticker);
    })[0] || null;
  }

  function applyFilters() {
    const tokens = tokenize(state.search);
    const hasSearch = tokens.length > 0;

    state.filtered = state.logos
      .filter((logo) => {
        if (state.exchange !== "all" && logo.exchange !== state.exchange) return false;
        if (state.sector !== "all" && logo.sector !== state.sector) return false;
        if (state.index === "constituents" && !logo.isIndexConstituent) return false;
        if (
          state.index !== "all" &&
          state.index !== "constituents" &&
          !(logo.indexes || []).includes(state.index)
        ) {
          return false;
        }
        if (!hasSearch) return true;
        return tokens.every((token) => logo.searchText.includes(token));
      })
      .map((logo) => ({
        ...logo,
        score: hasSearch ? scoreLogo(logo, tokens) : 0,
      }));

    sortFiltered(hasSearch);
    updateUrl();
    render();
  }

  function sortFiltered(hasSearch) {
    const sort = state.sort === "relevance" && !hasSearch ? "marketCapDesc" : state.sort;

    state.filtered.sort((a, b) => {
      if (sort === "relevance") return b.score - a.score || b.marketCapValue - a.marketCapValue;
      if (sort === "marketCapDesc") return b.marketCapValue - a.marketCapValue;
      if (sort === "marketCapAsc") return a.marketCapValue - b.marketCapValue;
      if (sort === "tickerAsc") return collator.compare(a.ticker, b.ticker);
      if (sort === "companyAsc") return collator.compare(a.company || "", b.company || "");
      if (sort === "sectorAsc") {
        return (
          collator.compare(a.sector || "", b.sector || "") ||
          collator.compare(a.ticker, b.ticker)
        );
      }
      return collator.compare(a.ticker, b.ticker);
    });
  }

  function scoreLogo(logo, tokens) {
    let score = 0;
    const ticker = normalize(logo.ticker);
    const company = normalize(logo.company || "");
    const sector = normalize(logo.sector || "");

    for (const token of tokens) {
      if (ticker === token) score += 120;
      else if (ticker.startsWith(token)) score += 75;
      else if (ticker.includes(token)) score += 40;

      if (company.startsWith(token)) score += 45;
      else if (company.includes(token)) score += 22;

      if (sector.includes(token)) score += 12;
    }
    return score;
  }

  function render() {
    const visibleLogos = state.filtered.slice(0, state.visible);
    els.results.replaceChildren();

    if (visibleLogos.length === 0) {
      showEmpty("No logos match this search.");
    } else {
      const fragment = document.createDocumentFragment();
      for (const logo of visibleLogos) {
        fragment.append(renderCard(logo));
      }
      els.results.append(fragment);
    }

    els.resultCount.textContent = `${visibleLogos.length} of ${state.filtered.length}`;
    els.selectedCount.textContent = String(state.selected.size);
    els.loadMore.hidden = state.visible >= state.filtered.length;
    els.downloadSelected.disabled = state.busy || state.selected.size === 0;
    els.downloadResults.disabled = state.busy || state.filtered.length === 0;
  }

  function renderCard(logo) {
    const card = els.template.content.firstElementChild.cloneNode(true);
    const checkbox = card.querySelector("input[type='checkbox']");
    const isSelected = state.selected.has(logo.id);

    card.classList.toggle("is-selected", isSelected);
    checkbox.checked = isSelected;
    checkbox.addEventListener("change", () => {
      if (checkbox.checked) state.selected.add(logo.id);
      else state.selected.delete(logo.id);
      card.classList.toggle("is-selected", checkbox.checked);
      els.selectedCount.textContent = String(state.selected.size);
      els.downloadSelected.disabled = state.busy || state.selected.size === 0;
    });

    card.querySelector("img").src = logo.file;
    card.querySelector("img").alt = `${logo.company || logo.ticker} logo`;
    card.querySelector(".exchange").textContent = logo.exchange;
    card.querySelector("h2").textContent = logo.ticker;
    card.querySelector(".company").textContent = logo.company || logo.ticker;
    card.querySelector(".industry").textContent = logo.industry || logo.oneLine || "Listed company";
    card.querySelector(".sector").textContent = logo.sector || "Unclassified";
    card.querySelector(".isin").textContent = logo.isin || "Not available";
    card.querySelector(".market-cap").textContent = logo.marketCapValue
      ? `${logo.currency || "INR"} ${numberFormatter.format(logo.marketCapValue)}`
      : "Not available";

    card.querySelector("a").href = `company/${symbolRef(logo)}`;

    card.querySelector("[data-action='download']").addEventListener("click", () => downloadLogo(logo));
    card.querySelector("[data-action='copy-url']").addEventListener("click", () => {
      copyText(apiUrl("logo", logo), `${logo.ticker} URL copied`);
    });

    return card;
  }

  function showEmpty(message) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.textContent = message;
    els.results.replaceChildren(empty);
  }

  async function downloadLogo(logo) {
    try {
      setBusy(true);
      const format = state.format;
      if (format === "svg") {
        const anchor = document.createElement("a");
        anchor.href = logo.file;
        anchor.download = fileNameFor(logo, "svg");
        anchor.click();
      } else {
        const blob = await rasterizeLogo(logo, format);
        saveBlob(blob, fileNameFor(logo, format));
      }

      toast(`Downloaded ${logo.ticker}.${format}`);
    } catch (error) {
      console.error(error);
      toast("Download failed for this logo.");
    } finally {
      setBusy(false);
    }
  }

  async function downloadBatch(logos, scope) {
    if (!logos.length) {
      toast(scope === "selected" ? "No selected logos." : "No logos in current results.");
      return;
    }

    const format = state.format;
    if (typeof window.JSZip !== "function") {
      toast("Bulk ZIP support is still loading. Try again in a moment.");
      return;
    }

    if (
      format !== "svg" &&
      logos.length > MAX_RASTER_WITHOUT_CONFIRM &&
      !window.confirm(`Convert ${logos.length} logos to ${format.toUpperCase()}? This can take a while.`)
    ) {
      return;
    }

    setBusy(true);
    const zip = new window.JSZip();
    const folder = zip.folder(`logos-${format}`);

    try {
      for (let index = 0; index < logos.length; index += 1) {
        const logo = logos[index];
        if (format === "svg") {
          const response = await fetch(logo.file);
          folder.file(fileNameFor(logo, "svg"), await response.text());
        } else {
          folder.file(fileNameFor(logo, format), await rasterizeLogo(logo, format));
        }

        if (index % 25 === 0 || index === logos.length - 1) {
          toast(`Preparing ${index + 1} of ${logos.length}`);
          await waitFrame();
        }
      }

      const blob = await zip.generateAsync(
        { type: "blob", compression: "DEFLATE", compressionOptions: { level: 6 } },
        (progress) => {
          if (progress.percent) {
            toast(`Compressing ${Math.round(progress.percent)}%`);
          }
        },
      );

      saveBlob(blob, `indian-listed-company-logos-${scope}-${format}.zip`);
      toast(`ZIP ready: ${logos.length} logos`);
    } catch (error) {
      console.error(error);
      toast("Bulk download failed.");
    } finally {
      setBusy(false);
    }
  }

  async function rasterizeLogo(logo, format) {
    const response = await fetch(logo.file);
    if (!response.ok) {
      throw new Error(`Could not fetch ${logo.file}`);
    }

    const svgText = await response.text();
    const svgBlob = new Blob([svgText], { type: "image/svg+xml;charset=utf-8" });
    const objectUrl = URL.createObjectURL(svgBlob);

    try {
      const image = await loadImage(objectUrl);
      const size = EXPORT_SIZE * state.scale;
      const canvas = document.createElement("canvas");
      canvas.width = size;
      canvas.height = size;

      const context = canvas.getContext("2d");
      if (format === "jpg") {
        context.fillStyle = "#ffffff";
        context.fillRect(0, 0, size, size);
      }

      const dimensions = fitWithin(image.naturalWidth || size, image.naturalHeight || size, size * 0.82);
      context.drawImage(
        image,
        (size - dimensions.width) / 2,
        (size - dimensions.height) / 2,
        dimensions.width,
        dimensions.height,
      );

      return await canvasToBlob(canvas, format);
    } finally {
      URL.revokeObjectURL(objectUrl);
    }
  }

  function loadImage(src) {
    return new Promise((resolve, reject) => {
      const image = new Image();
      image.onload = () => resolve(image);
      image.onerror = reject;
      image.src = src;
    });
  }

  function canvasToBlob(canvas, format) {
    const type = format === "jpg" ? "image/jpeg" : "image/png";
    const quality = format === "jpg" ? state.quality : undefined;
    return new Promise((resolve, reject) => {
      canvas.toBlob(
        (blob) => {
          if (blob) resolve(blob);
          else reject(new Error("Canvas export failed"));
        },
        type,
        quality,
      );
    });
  }

  function fitWithin(width, height, maxSize) {
    const ratio = Math.min(maxSize / width, maxSize / height);
    return {
      width: Math.max(1, Math.round(width * ratio)),
      height: Math.max(1, Math.round(height * ratio)),
    };
  }

  function fileNameFor(logo, extension) {
    const base = logo.file.split("/").pop().replace(/\.svg$/i, "");
    return `${base}.${extension}`;
  }

  function saveBlob(blob, fileName) {
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = fileName;
    anchor.click();
    setTimeout(() => URL.revokeObjectURL(url), 1500);
  }

  function updateUrl() {
    const params = new URLSearchParams();
    if (state.search) params.set("q", state.search);
    if (state.exchange !== "all") params.set("exchange", state.exchange);
    if (state.sector !== "all") params.set("sector", state.sector);
    if (state.index !== "all") params.set("index", state.index);
    if (state.sort !== "relevance") params.set("sort", state.sort);
    if (state.apiTicker) {
      const parsedApi = parseTickerInput(state.apiTicker);
      params.set("ticker", parsedApi.ticker || state.apiTicker);
    }
    if (state.apiExchange !== "auto") params.set("apiExchange", state.apiExchange);

    const nextUrl = params.toString()
      ? `${window.location.pathname}?${params.toString()}`
      : window.location.pathname;
    window.history.replaceState({}, "", nextUrl);
  }

  function setBusy(isBusy) {
    state.busy = isBusy;
    document.body.classList.toggle("is-busy", isBusy);
    els.downloadSelected.disabled = isBusy || state.selected.size === 0;
    els.downloadResults.disabled = isBusy || state.filtered.length === 0;
  }

  let toastTimer = 0;
  function toast(message) {
    window.clearTimeout(toastTimer);
    els.toast.textContent = message;
    els.toast.classList.add("is-visible");
    toastTimer = window.setTimeout(() => els.toast.classList.remove("is-visible"), 2600);
  }

  function copyTarget(targetId) {
    const target = document.getElementById(targetId);
    if (!target) return;
    const value = "value" in target ? target.value : target.textContent;
    copyText(value, "Copied to clipboard");
  }

  async function copyText(value, successMessage) {
    if (!value) return;

    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(value);
      } else {
        fallbackCopy(value);
      }
      toast(successMessage);
    } catch (error) {
      console.error(error);
      toast("Copy failed.");
    }
  }

  function fallbackCopy(value) {
    const textarea = document.createElement("textarea");
    textarea.value = value;
    textarea.setAttribute("readonly", "");
    textarea.style.position = "fixed";
    textarea.style.top = "-1000px";
    textarea.style.left = "-1000px";
    document.body.append(textarea);
    textarea.select();
    const copied = document.execCommand("copy");
    textarea.remove();
    if (!copied) throw new Error("Copy command failed");
  }

  function refreshApiCopyButtons() {
    els.apiCopyButtons.forEach((button) => {
      const target = document.getElementById(button.dataset.copyTarget);
      const value = target && "value" in target ? target.value : target?.textContent;
      button.disabled = !value;
    });
  }

  function absoluteUrl(path) {
    return new URL(path, window.location.href).href;
  }

  // "NSE:TCS" style reference, the same one the API uses in its own links.
  function symbolRef(logo) {
    return encodeURIComponent(logo.id).replace(/%3A/g, ":");
  }

  function apiUrl(resource, logo) {
    return absoluteUrl(`v1/${resource}/${symbolRef(logo)}`);
  }

  function escapeAttribute(value) {
    return String(value || "")
      .replace(/&/g, "&amp;")
      .replace(/"/g, "&quot;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  function parseTickerInput(value) {
    let ticker = String(value || "").trim().toUpperCase();
    let exchange = "";

    if (!ticker) {
      return { ticker: "", exchange: "" };
    }

    const exchangePrefix = ticker.match(/^(NSE|BSE)\s*[:/_-]\s*(.+)$/);
    if (exchangePrefix) {
      exchange = exchangePrefix[1];
      ticker = exchangePrefix[2];
    }

    const yahooSuffix = ticker.match(/^(.+)\.(NS|BO)$/);
    if (yahooSuffix) {
      ticker = yahooSuffix[1];
      exchange = yahooSuffix[2] === "NS" ? "NSE" : "BSE";
    }

    return {
      ticker: ticker.replace(/\s+/g, ""),
      exchange,
    };
  }

  function normalizeTicker(value) {
    return String(value || "").trim().toUpperCase();
  }

  function normalize(value) {
    return String(value || "")
      .toLowerCase()
      .normalize("NFKD")
      .replace(/[\u0300-\u036f]/g, "");
  }

  function tokenize(value) {
    return normalize(value)
      .split(/[^a-z0-9&.]+/i)
      .map((token) => token.trim())
      .filter(Boolean);
  }

  function debounce(callback, delay) {
    let timer = 0;
    return (...args) => {
      window.clearTimeout(timer);
      timer = window.setTimeout(() => callback(...args), delay);
    };
  }

  function waitFrame() {
    return new Promise((resolve) => requestAnimationFrame(resolve));
  }
})();
