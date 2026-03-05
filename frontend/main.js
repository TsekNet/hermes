(function() {
  "use strict";

  var remaining = 300;
  var timeoutValue = "";
  var escValue = "";
  var responded = false;
  var timer = null;
  var Backend = null;
  var InboxBackend = null;
  var carouselIndex = 0;
  var carouselTotal = 0;

  var validStyles = { primary: true, secondary: true, danger: true };

  function findBackend(method) {
    if (!window.go) return null;
    for (var ns in window.go) {
      if (!window.go[ns]) continue;
      for (var cls in window.go[ns]) {
        if (window.go[ns][cls] && typeof window.go[ns][cls][method] === "function") {
          return window.go[ns][cls];
        }
      }
    }
    return null;
  }

  function init() {
    InboxBackend = findBackend("GetHistory");
    if (InboxBackend) { initInbox(); return; }
    Backend = findBackend("GetConfig");
    if (Backend) { Backend.GetConfig().then(configure); return; }

    var attempts = 0;
    var poll = setInterval(function() {
      attempts++;
      InboxBackend = findBackend("GetHistory");
      if (InboxBackend) { clearInterval(poll); initInbox(); return; }
      Backend = findBackend("GetConfig");
      if (Backend) { clearInterval(poll); Backend.GetConfig().then(configure); return; }
      if (attempts > 50) clearInterval(poll);
    }, 100);
  }

  function initInbox() {
    document.body.innerHTML = "";
    document.body.className = "inbox-body";

    var accent = document.createElement("div");
    accent.className = "accent";
    document.body.appendChild(accent);

    var container = document.createElement("div");
    container.className = "inbox-container";

    var header = document.createElement("div");
    header.className = "inbox-header";
    var title = document.createElement("span");
    title.className = "title-text";
    title.textContent = "NOTIFICATION HISTORY";
    header.appendChild(title);
    container.appendChild(header);

    var list = document.createElement("div");
    list.className = "inbox-list";
    list.id = "inbox-list";
    container.appendChild(list);

    document.body.appendChild(container);

    InboxBackend.GetHistory().then(function(entries) {
      renderInbox(entries || []);
      InboxBackend.Ready();
    }).catch(function(err) {
      console.error("GetHistory failed:", err);
      var list = document.getElementById("inbox-list");
      list.textContent = "Error loading history: " + err;
      InboxBackend.Ready();
    });
  }

  function renderInbox(entries) {
    var list = document.getElementById("inbox-list");
    if (entries.length === 0) {
      var empty = document.createElement("div");
      empty.className = "inbox-empty";
      empty.textContent = "No notification history";
      list.appendChild(empty);
      return;
    }
    for (var i = 0; i < entries.length; i++) {
      list.appendChild(buildInboxCard(entries[i]));
    }
  }

  function buildInboxCard(entry) {
    var card = document.createElement("div");
    card.className = "inbox-card";

    var top = document.createElement("div");
    top.className = "inbox-card-top";

    var heading = document.createElement("span");
    heading.className = "inbox-card-heading";
    heading.textContent = entry.heading;
    top.appendChild(heading);

    var actionable = entry.responseValue && entry.responseValue !== "dismiss"
        && entry.responseValue !== "cancelled" && entry.responseValue.indexOf("timeout") !== 0;

    var badge = document.createElement(actionable ? "button" : "span");
    badge.className = "inbox-card-badge";
    var label = entry.responseValue.replace(/_/g, " ");
    badge.textContent = label.charAt(0).toUpperCase() + label.slice(1);
    if (entry.responseValue === "dismiss" || entry.responseValue === "cancelled") {
      badge.classList.add("badge-muted");
    } else if (entry.responseValue.indexOf("timeout") === 0) {
      badge.classList.add("badge-warn");
    } else {
      badge.classList.add("badge-ok");
      badge.classList.add("badge-action");
    }
    if (actionable) {
      badge.setAttribute("data-id", entry.id);
      badge.setAttribute("data-value", entry.responseValue);
      badge.addEventListener("click", function(e) {
        var b = e.target;
        var id = b.getAttribute("data-id");
        var val = b.getAttribute("data-value");
        b.disabled = true;
        b.textContent = "Running...";
        InboxBackend.RunAction(id, val).then(function(result) {
          b.textContent = result === "ok" ? "Done" : result;
        }).catch(function() {
          b.textContent = "Failed";
        });
      });
    }
    top.appendChild(badge);
    card.appendChild(top);

    if (entry.message) {
      var msg = document.createElement("div");
      msg.className = "inbox-card-message";
      msg.textContent = entry.message;
      card.appendChild(msg);
    }

    var meta = document.createElement("div");
    meta.className = "inbox-card-meta";
    var parts = [];
    if (entry.source) parts.push(entry.source);
    if (entry.completedAt) {
      var d = new Date(entry.completedAt);
      parts.push(d.toLocaleDateString() + " " + d.toLocaleTimeString([], {hour: "2-digit", minute: "2-digit"}));
    }
    meta.textContent = parts.join("  \u00B7  ");
    card.appendChild(meta);

    return card;
  }

  function configure(cfg) {
    if (!cfg) return;

    if (cfg.accentColor && /^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$/.test(cfg.accentColor)) {
      document.documentElement.style.setProperty("--accent", cfg.accentColor);
      document.documentElement.style.setProperty("--btn-primary-bg", cfg.accentColor);
    }

    document.getElementById("title").textContent = cfg.title || "";
    document.getElementById("heading").textContent = cfg.heading || "";
    document.getElementById("message").textContent = cfg.message || "";

    if (cfg.helpUrl && /^https?:\/\//i.test(cfg.helpUrl)) {
      var link = document.getElementById("help-link");
      link.href = cfg.helpUrl;
      link.style.display = "inline";
      link.addEventListener("click", function(e) {
        e.preventDefault();
        if (Backend) Backend.OpenHelp();
      });
    }

    if (cfg.images && cfg.images.length > 0) {
      buildCarousel(cfg.images);
    }

    if (cfg.watchPaths && cfg.watchPaths.length > 0) {
      initWatchStatus();
    }

    remaining = cfg.timeout || 300;
    timeoutValue = cfg.timeoutValue || "";
    escValue = cfg.escValue || timeoutValue;

    buildButtons(cfg.buttons || []);
    startCountdown();

    if (Backend) Backend.Ready();
  }

  function buildCarousel(images) {
    var carousel = document.getElementById("carousel");
    var track = document.getElementById("carousel-track");
    var controls = document.getElementById("carousel-controls");

    carousel.style.display = "block";
    carouselTotal = images.length;
    carouselIndex = 0;

    for (var i = 0; i < images.length; i++) {
      var img = document.createElement("img");
      img.src = images[i];
      img.alt = "";
      img.draggable = false;
      img.onerror = function() {
        this.style.display = "none";
        var ph = document.createElement("div");
        ph.className = "carousel-placeholder";
        ph.textContent = "Image unavailable";
        this.parentNode.insertBefore(ph, this.nextSibling);
      };
      track.appendChild(img);
    }

    if (images.length > 1) {
      controls.style.display = "flex";
      document.getElementById("carousel-prev").addEventListener("click", carouselPrev);
      document.getElementById("carousel-next").addEventListener("click", carouselNext);
      updateCarousel();
    }
  }

  function carouselPrev() {
    carouselIndex = (carouselIndex - 1 + carouselTotal) % carouselTotal;
    updateCarousel();
  }

  function carouselNext() {
    carouselIndex = (carouselIndex + 1) % carouselTotal;
    updateCarousel();
  }

  function updateCarousel() {
    var track = document.getElementById("carousel-track");
    var imgs = track.querySelectorAll("img");
    for (var i = 0; i < imgs.length; i++) {
      imgs[i].style.transform = "translateX(-" + (carouselIndex * 100) + "%)";
    }
    document.getElementById("carousel-indicator").textContent =
      (carouselIndex + 1) + " / " + carouselTotal;
  }

  function initWatchStatus() {
    var el = document.getElementById("watch-status");
    el.style.display = "block";
    el.textContent = "Monitoring filesystem...";

    if (window.runtime && window.runtime.EventsOn) {
      window.runtime.EventsOn("fs:event", function(ev) {
        var basename = ev.path.split("/").pop().split("\\").pop();
        el.textContent = ev.op + ": " + basename;
      });
    }
  }

  function buildButtons(buttons) {
    var container = document.getElementById("buttons");
    container.innerHTML = "";
    for (var i = 0; i < buttons.length; i++) {
      var btn = buttons[i];
      if (btn.dropdown && btn.dropdown.length > 0) {
        container.appendChild(buildDropdown(btn));
      } else {
        var el = document.createElement("button");
        el.className = "btn btn-" + (validStyles[btn.style] ? btn.style : "secondary");
        el.textContent = btn.label;
        el.setAttribute("data-value", btn.value || btn.label.toLowerCase());
        el.addEventListener("click", onButtonClick);
        container.appendChild(el);
      }
    }
  }

  function buildDropdown(btn) {
    var wrapper = document.createElement("div");
    wrapper.className = "dropdown-wrapper";

    var trigger = document.createElement("button");
    trigger.className = "btn btn-" + (validStyles[btn.style] ? btn.style : "secondary");
    trigger.textContent = btn.label + " \u25B4";
    trigger.addEventListener("click", function(e) {
      e.stopPropagation();
      wrapper.querySelector(".dropdown-menu").classList.toggle("open");
    });
    wrapper.appendChild(trigger);

    var menu = document.createElement("div");
    menu.className = "dropdown-menu";
    for (var j = 0; j < btn.dropdown.length; j++) {
      var opt = btn.dropdown[j];
      var item = document.createElement("div");
      item.className = "dropdown-item";
      item.textContent = opt.label;
      item.setAttribute("data-value", opt.value || opt.label || "");
      item.addEventListener("click", onButtonClick);
      menu.appendChild(item);
    }
    wrapper.appendChild(menu);
    return wrapper;
  }

  function onButtonClick(e) {
    respond(e.target.getAttribute("data-value"));
  }

  function respond(value) {
    if (responded) return;
    responded = true;
    if (timer) clearInterval(timer);
    if (Backend) Backend.Respond(value);
  }

  function startCountdown() {
    updateCountdown();
    timer = setInterval(function() {
      remaining--;
      if (remaining <= 0) { clearInterval(timer); respond("timeout:" + timeoutValue); return; }
      updateCountdown();
    }, 1000);
  }

  function updateCountdown() {
    var el = document.getElementById("countdown");
    if (remaining <= 0) { el.textContent = ""; return; }
    var m = Math.floor(remaining / 60);
    var s = remaining % 60;
    el.textContent = "Auto-action in " + m + ":" + (s < 10 ? "0" : "") + s;
  }

  document.addEventListener("click", function() {
    var menus = document.querySelectorAll(".dropdown-menu.open");
    for (var i = 0; i < menus.length; i++) menus[i].classList.remove("open");
  });

  document.addEventListener("keydown", function(e) {
    if (!document.hasFocus()) return;
    if (e.keyCode === 27) respond(escValue);
    if (e.keyCode === 13) {
      var primary = document.querySelector(".btn-primary");
      if (primary) respond(primary.getAttribute("data-value"));
    }
    if (carouselTotal > 1) {
      if (e.keyCode === 37) carouselPrev();
      if (e.keyCode === 39) carouselNext();
    }
  });

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
