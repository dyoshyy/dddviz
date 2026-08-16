// dddviz rendering. ELK computes the coordinates; this file builds the SVG.
//
// An aggregate is a container node and its contents are that node's children.
// The contents have no edges between them, so the inside of a container is
// packed with rectpacking rather than layered: a layered pass would stack
// edgeless children into one column and make the box needlessly tall.
(function () {
  "use strict";

  var graph = window.__DDDVIZ__;
  var elk = new ELK();

  var CHAR = 6.7; // measured width of the 11px monospace face
  var TITLE_CHAR = 7.4; // 12-13px sans-serif
  var ROW = 16;
  var HEAD = 24;
  var PAD = 9;
  var MIN_W = 130;

  // Expansion state. Everything starts collapsed, so the first view is the
  // map between aggregates.
  var expanded = new Set();

  var stage = document.getElementById("stage");
  var view = { x: 0, y: 0, k: 1 };
  var focus = null;

  // ---- sizing ------------------------------------------------------

  function fieldWidth(f) {
    return (f.name.length + f.type.length + 2) * CHAR;
  }

  function widestField(fields) {
    var w = 0;
    for (var i = 0; i < fields.length; i++) w = Math.max(w, fieldWidth(fields[i]));
    return w;
  }

  function memberSize(m) {
    var title = (m.name.length + m.kind.length + 3) * TITLE_CHAR;
    var w = Math.max(title, widestField(m.fields)) + PAD * 2;
    var h = HEAD + (m.fields.length ? m.fields.length * ROW + 6 : 0) + 8;
    return { width: Math.max(MIN_W, Math.ceil(w)), height: Math.ceil(h) };
  }

  // Height of an aggregate header: the name plus the root's own field rows.
  function aggHeadHeight(agg) {
    return HEAD + (agg.fields.length ? agg.fields.length * ROW + 6 : 0) + 8;
  }

  function aggCollapsedSize(agg) {
    var title = (agg.name.length + (agg.idType || "").length + 4) * TITLE_CHAR;
    var w = Math.max(title, widestField(agg.fields)) + PAD * 2;
    var count = agg.members.length ? plural(agg.members.length, "type") : "";
    return {
      width: Math.max(MIN_W + 30, Math.ceil(w), count.length * CHAR + PAD * 2),
      height: aggHeadHeight(agg) + (agg.members.length ? ROW + 4 : 0),
    };
  }

  // ---- building the ELK graph ---------------------------------------

  function buildElk() {
    var children = graph.aggregates.map(function (agg) {
      var head = aggHeadHeight(agg);
      var node = { id: "agg:" + agg.name, dddType: "agg", ddd: agg };

      if (expanded.has(agg.name) && agg.members.length) {
        node.children = agg.members.map(function (m, i) {
          var s = memberSize(m);
          return {
            id: "mem:" + agg.name + ":" + i,
            width: s.width,
            height: s.height,
            dddType: "member",
            ddd: m,
            aggName: agg.name,
          };
        });
        node.layoutOptions = {
          "elk.algorithm": "rectpacking",
          "elk.aspectRatio": "1.7",
          "elk.spacing.nodeNode": "12",
          "elk.padding":
            "[top=" + (head + 6) + ",left=12,bottom=12,right=12]",
        };
      } else {
        var s = aggCollapsedSize(agg);
        node.width = s.width;
        node.height = s.height;
      }
      return node;
    });

    var edges = graph.references.map(function (r, i) {
      return {
        id: "ref:" + i,
        sources: ["agg:" + r.from],
        targets: ["agg:" + r.to],
        ddd: r,
      };
    });

    return {
      id: "root",
      layoutOptions: {
        "elk.algorithm": "layered",
        "elk.direction": "RIGHT",
        "elk.hierarchyHandling": "INCLUDE_CHILDREN",
        "elk.edgeRouting": "ORTHOGONAL",
        "elk.spacing.nodeNode": "45",
        "elk.layered.spacing.nodeNodeBetweenLayers": "80",
        "elk.layered.spacing.edgeNodeBetweenLayers": "30",
        "elk.padding": "[top=30,left=30,bottom=30,right=30]",
      },
      children: children,
      edges: edges,
    };
  }

  // ---- building the SVG ----------------------------------------------

  var NS = "http://www.w3.org/2000/svg";

  function el(name, attrs, parent) {
    var n = document.createElementNS(NS, name);
    for (var k in attrs) {
      if (attrs[k] !== undefined && attrs[k] !== null) n.setAttribute(k, attrs[k]);
    }
    if (parent) parent.appendChild(n);
    return n;
  }

  function text(parent, x, y, cls, content) {
    var t = el("text", { x: x, y: y, class: cls }, parent);
    t.textContent = content;
    return t;
  }

  // Draw the field rows, colouring the name apart from the type.
  function drawFields(g, fields, x, y) {
    for (var i = 0; i < fields.length; i++) {
      var row = el("text", { x: x, y: y + i * ROW, class: "field" }, g);
      var n = el("tspan", { class: "field-name" }, row);
      n.textContent = fields[i].name;
      var t = el("tspan", {}, row);
      t.textContent = "  " + fields[i].type;
    }
  }

  function drawAggregate(parent, node) {
    var agg = node.ddd;
    var isOpen = expanded.has(agg.name) && agg.members.length > 0;
    var g = el(
      "g",
      {
        class: "agg node",
        transform: "translate(" + node.x + "," + node.y + ")",
        "data-agg": agg.name,
      },
      parent
    );

    el("rect", { width: node.width, height: node.height }, g);

    var head = aggHeadHeight(agg);
    el(
      "path",
      {
        class: "agg-head",
        d:
          "M0,6 a6,6 0 0 1 6,-6 h" +
          (node.width - 12) +
          " a6,6 0 0 1 6,6 v" +
          (head - 6) +
          " h" +
          -node.width +
          " z",
      },
      g
    );
    el("line", { x1: 0, y1: head, x2: node.width, y2: head, class: "rule" }, g);

    text(g, PAD, 17, "agg-title", agg.name);
    if (agg.idType) {
      var idt = text(g, node.width - PAD, 17, "agg-id", agg.idType);
      idt.setAttribute("text-anchor", "end");
    }
    if (agg.fields.length) drawFields(g, agg.fields, PAD, HEAD + 12);

    if (!isOpen && agg.members.length) {
      text(g, PAD, head + ROW, "badge", plural(agg.members.length, "type") + " \u25b8");
    }
    return g;
  }

  function drawMember(parent, node) {
    var m = node.ddd;
    var g = el(
      "g",
      {
        class: "member node " + m.kind,
        transform: "translate(" + node.x + "," + node.y + ")",
        "data-type": m.name,
      },
      parent
    );
    el("rect", { width: node.width, height: node.height }, g);
    text(g, PAD, 16, "member-title", m.name);
    var b = text(g, node.width - PAD, 16, "badge", m.kind.toUpperCase());
    b.setAttribute("text-anchor", "end");
    if (m.fields.length) {
      el(
        "line",
        { x1: PAD, y1: HEAD - 2, x2: node.width - PAD, y2: HEAD - 2, class: "rule" },
        g
      );
      drawFields(g, m.fields, PAD, HEAD + 12);
    }
    return g;
  }

  function drawEdges(parent, node, ox, oy) {
    (node.edges || []).forEach(function (e) {
      var g = el("g", { class: "edge", "data-from": e.ddd.from, "data-to": e.ddd.to }, parent);
      (e.sections || []).forEach(function (s) {
        var d = "M" + (s.startPoint.x + ox) + "," + (s.startPoint.y + oy);
        (s.bendPoints || []).forEach(function (p) {
          d += "L" + (p.x + ox) + "," + (p.y + oy);
        });
        d += "L" + (s.endPoint.x + ox) + "," + (s.endPoint.y + oy);
        el("path", { d: d, "marker-end": "url(#arrow)" }, g);

        var mid = (s.bendPoints && s.bendPoints.length)
          ? s.bendPoints[Math.floor(s.bendPoints.length / 2)]
          : { x: (s.startPoint.x + s.endPoint.x) / 2, y: (s.startPoint.y + s.endPoint.y) / 2 };
        var t = text(g, mid.x + ox, mid.y + oy - 6, "edge-label", e.ddd.via);
        t.setAttribute("text-anchor", "middle");
      });
    });
  }

  function render(layout) {
    stage.innerHTML = "";
    var svg = el("svg", {}, stage);
    var defs = el("defs", {}, svg);
    var mk = el(
      "marker",
      {
        id: "arrow",
        viewBox: "0 0 10 10",
        refX: "9",
        refY: "5",
        markerWidth: "7",
        markerHeight: "7",
        orient: "auto-start-reverse",
      },
      defs
    );
    el("path", { d: "M0,1 L9,5 L0,9 z", fill: "var(--edge)" }, mk);

    var root = el("g", { id: "camera" }, svg);

    // Draw the edges first so they sit underneath the nodes.
    drawEdges(root, layout, 0, 0);

    layout.children.forEach(function (aggNode) {
      var g = drawAggregate(root, aggNode);
      (aggNode.children || []).forEach(function (memNode) {
        drawMember(g, memNode);
      });
      drawEdges(root, aggNode, aggNode.x, aggNode.y);
    });

    lastLayout = layout;
    applyView();
    bindNodes(svg);
    if (refit) {
      refit = false;
      fit(layout);
    }
  }

  // ---- viewport -------------------------------------------------------

  function applyView() {
    var cam = document.getElementById("camera");
    if (cam) {
      cam.setAttribute(
        "transform",
        "translate(" + view.x + "," + view.y + ") scale(" + view.k + ")"
      );
    }
  }

  // Expanding one aggregate keeps the viewport where it is; only the
  // whole-diagram actions refit.
  var refit = true;
  var lastLayout = null;

  function fit(layout) {
    var w = stage.clientWidth, h = stage.clientHeight;
    if (!layout.width || !layout.height || !w || !h) return;
    var k = Math.min(w / (layout.width + 40), h / (layout.height + 40), 1.4);
    view.k = k;
    view.x = (w - layout.width * k) / 2;
    view.y = (h - layout.height * k) / 2;
    applyView();
  }

  // Stage-level handlers are bound once. Rebinding them on every redraw
  // would pile listeners up and slow the page down.
  function bindStage() {
    var drag = null;

    stage.addEventListener("mousedown", function (e) {
      if (e.button !== 0) return;
      drag = { x: e.clientX - view.x, y: e.clientY - view.y, moved: false };
      stage.classList.add("dragging");
    });
    window.addEventListener("mousemove", function (e) {
      if (!drag) return;
      view.x = e.clientX - drag.x;
      view.y = e.clientY - drag.y;
      drag.moved = true;
      applyView();
    });
    window.addEventListener("mouseup", function () {
      drag = null;
      stage.classList.remove("dragging");
    });

    stage.addEventListener(
      "wheel",
      function (e) {
        e.preventDefault();
        var r = stage.getBoundingClientRect();
        var mx = e.clientX - r.left, my = e.clientY - r.top;
        var f = Math.exp(-e.deltaY * 0.0016);
        var k = Math.min(3, Math.max(0.15, view.k * f));
        view.x = mx - (mx - view.x) * (k / view.k);
        view.y = my - (my - view.y) * (k / view.k);
        view.k = k;
        applyView();
      },
      { passive: false }
    );

    // Press f to fit the whole diagram on screen.
    window.addEventListener("keydown", function (e) {
      if (e.key === "f" && lastLayout) fit(lastLayout);
    });
  }

  // Node-level handlers are rebound on every redraw.
  function bindNodes(svg) {
    // Clicking an aggregate expands or collapses it; clicks on its
    // contents do not count.
    svg.querySelectorAll(".agg").forEach(function (g) {
      g.addEventListener("click", function (e) {
        if (e.target.closest(".member")) return;
        var name = g.getAttribute("data-agg");
        if (expanded.has(name)) expanded.delete(name);
        else expanded.add(name);
        layout();
      });
    });

    // Hovering keeps the related edges and aggregates lit and dims the rest.
    svg.querySelectorAll(".node").forEach(function (g) {
      g.addEventListener("mouseenter", function () {
        var agg = g.classList.contains("agg")
          ? g.getAttribute("data-agg")
          : g.closest(".agg").getAttribute("data-agg");
        highlight(svg, agg);
      });
      g.addEventListener("mouseleave", function () {
        clearHighlight(svg);
      });
    });
  }

  function highlight(svg, aggName) {
    var keep = new Set([aggName]);
    svg.querySelectorAll(".edge").forEach(function (e) {
      var f = e.getAttribute("data-from"), t = e.getAttribute("data-to");
      if (f === aggName || t === aggName) {
        keep.add(f);
        keep.add(t);
        e.classList.add("hl");
      } else {
        e.classList.add("fade");
      }
    });
    svg.querySelectorAll(".agg").forEach(function (g) {
      var n = g.getAttribute("data-agg");
      if (keep.has(n)) {
        if (n === aggName) g.classList.add("hl");
      } else {
        g.classList.add("fade");
      }
    });
    svg.classList.add("has-focus");
  }

  function clearHighlight(svg) {
    svg.classList.remove("has-focus");
    svg.querySelectorAll(".fade, .hl").forEach(function (n) {
      n.classList.remove("fade", "hl");
    });
  }

  // ---- entry ----------------------------------------------------------

  function layout() {
    elk
      .layout(buildElk())
      .then(render)
      .catch(function (err) {
        stage.innerHTML =
          '<pre style="padding:20px;color:var(--accent)">layout failed: ' +
          String(err) +
          "</pre>";
      });
  }

  function buildSide() {
    var side = document.getElementById("side");
    var aggCount = graph.aggregates.length;
    var refCount = graph.references.length;

    var html =
      "<h1>" + escapeHtml(graph.meta.title) + "</h1>" +
      '<div class="sub">' + plural(aggCount, "aggregate") +
      " / " + plural(refCount, "reference") + "</div>";

    html +=
      '<h2>Controls</h2><div class="hint">' +
      "Click an aggregate to expand or collapse<br>" +
      "Hover to highlight what it relates to<br>" +
      "Drag to pan, <kbd>wheel</kbd> to zoom<br>" +
      "<kbd>f</kbd> to fit on screen<br><br>" +
      '<button class="link" id="all-open">Expand all</button> / ' +
      '<button class="link" id="all-close">Collapse all</button>' +
      "</div>";

    if (graph.unclassified.length) {
      html +=
        "<h2>Unclassified " + graph.unclassified.length + "</h2><ul>" +
        graph.unclassified
          .map(function (u) {
            return "<li><b>" + escapeHtml(u.name) + "</b><br>" + escapeHtml(u.pos) + "</li>";
          })
          .join("") +
        "</ul>" +
        '<div class="hint" style="margin-top:8px">Types no aggregate root can reach. ' +
        "Usually domain services and DTOs, but a forgotten marker shows up here too.</div>";
    }

    side.innerHTML = html;

    // These act on the whole diagram, so refit after redrawing.
    document.getElementById("all-open").addEventListener("click", function () {
      graph.aggregates.forEach(function (a) { expanded.add(a.name); });
      refit = true;
      layout();
    });
    document.getElementById("all-close").addEventListener("click", function () {
      expanded.clear();
      refit = true;
      layout();
    });
  }

  function plural(n, noun) {
    return n + " " + noun + (n === 1 ? "" : "s");
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
    });
  }

  // ---- live reload (-watch) -------------------------------------------

  // The server sends a new graph rather than telling the page to reload, so
  // the expanded aggregates survive an edit. Names that no longer exist are
  // simply never looked up again.
  function connectLive() {
    var src = new EventSource("/events");

    src.addEventListener("graph", function (e) {
      graph = JSON.parse(e.data);
      clearBanner();
      buildSide();
      layout();
    });

    src.addEventListener("failed", function (e) {
      // Code mid-edit does not compile. Keep the last good diagram on
      // screen and say why it is not moving.
      showBanner(JSON.parse(e.data));
    });

    src.addEventListener("error", function () {
      showBanner("Disconnected from dddviz. Is the -watch process still running?");
    });
  }

  function showBanner(message) {
    var b = document.getElementById("banner");
    if (!b) {
      b = document.createElement("div");
      b.id = "banner";
      document.getElementById("app").appendChild(b);
    }
    b.textContent = firstUsefulLine(message);
    b.title = message;
  }

  // A build failure leads with a heading and the package name; the line
  // worth showing is the first one that names a file and a position.
  function firstUsefulLine(message) {
    var lines = String(message).split("\n");
    for (var i = 0; i < lines.length; i++) {
      var line = lines[i].trim();
      if (/\.go:\d+(:\d+)?:/.test(line)) {
        // Strip any leading directory noise so the file name stays visible
        // when the banner has to truncate.
        return line.replace(/^.*?([^/\\]+\.go:\d+)/, "$1");
      }
    }
    return lines[0];
  }

  function clearBanner() {
    var b = document.getElementById("banner");
    if (b) b.remove();
  }

  buildSide();
  bindStage();
  layout();
  if (window.__DDDVIZ_LIVE__) connectLive();
})();
