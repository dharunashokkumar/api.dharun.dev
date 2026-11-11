// The logo search on /logos: companies by name or ticker, each linked to its page.
(function () {
  "use strict";
  var form = document.getElementById("logo-find"), out = document.getElementById("logo-results");
  if (!form) return;
  form.addEventListener("submit", async function (event) {
    event.preventDefault();
    var q = form.elements.q.value.trim();
    if (!q) return;
    var response = await fetch("/v1/companies?limit=24&q=" + encodeURIComponent(q)), data = await response.json();
    out.textContent = "";
    var wall = document.createElement("div");
    wall.className = "logo-wall";
    (data.data || []).forEach(function (c) {
      var figure = document.createElement("figure"), link = document.createElement("a"), img = document.createElement("img"), caption = document.createElement("figcaption");
      link.href = "/logos/" + (c.exchange === "NSE" ? encodeURIComponent(c.ticker) : c.id);
      img.src = c.logo.svg; img.alt = c.company + " logo"; img.width = 48; img.height = 48; img.loading = "lazy";
      caption.textContent = c.ticker;
      link.appendChild(img); figure.appendChild(link); figure.appendChild(caption); wall.appendChild(figure);
    });
    if (!wall.children.length) { var none = document.createElement("p"); none.className = "note"; none.textContent = "No listed company matches."; out.appendChild(none); }
    out.appendChild(wall);
  });
})();
