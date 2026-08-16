// dddviz の描画。ELK に座標を計算させ、SVG を組み立てる。
//
// 集約はコンテナノード、中身はその子ノード。中身どうしは辺を持たないので
// コンテナ内は layered ではなく rectpacking で詰める。層状にすると
// 辺のない子が一列に並んで無駄に縦長になる。
(function () {
  "use strict";

  var graph = window.__DDDVIZ__;
  var elk = new ELK();

  var CHAR = 6.7; // 11px 等幅の実測近似
  var TITLE_CHAR = 7.4; // 12-13px サンセリフ
  var ROW = 16;
  var HEAD = 24;
  var PAD = 9;
  var MIN_W = 130;

  // 展開状態。初期は全部たたんで集約間の地図から始める。
  var expanded = new Set();

  var stage = document.getElementById("stage");
  var view = { x: 0, y: 0, k: 1 };
  var focus = null;

  // ---- サイズ計算 -------------------------------------------------

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

  // 集約ヘッダの高さ。名前と、ルート自身のフィールド行を収める。
  function aggHeadHeight(agg) {
    return HEAD + (agg.fields.length ? agg.fields.length * ROW + 6 : 0) + 8;
  }

  function aggCollapsedSize(agg) {
    var title = (agg.name.length + (agg.idType || "").length + 4) * TITLE_CHAR;
    var w = Math.max(title, widestField(agg.fields)) + PAD * 2;
    var count = agg.members.length ? agg.members.length + " 型" : "";
    return {
      width: Math.max(MIN_W + 30, Math.ceil(w), count.length * CHAR + PAD * 2),
      height: aggHeadHeight(agg) + (agg.members.length ? ROW + 4 : 0),
    };
  }

  // ---- ELK グラフの組み立て ---------------------------------------

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

  // ---- SVG 組み立て -----------------------------------------------

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

  // フィールド行を描く。名前と型で色を分ける。
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
      text(g, PAD, head + ROW, "badge", agg.members.length + " 型 ▸");
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

    // 辺を先に描いてノードの下に敷く。
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

  // ---- ビュー操作 --------------------------------------------------

  function applyView() {
    var cam = document.getElementById("camera");
    if (cam) {
      cam.setAttribute(
        "transform",
        "translate(" + view.x + "," + view.y + ") scale(" + view.k + ")"
      );
    }
  }

  // 個別の展開では視点を保ち、全体を見る操作のときだけ収め直す。
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

  // ステージ側の操作は一度だけ結ぶ。描き直しのたびに足すと
  // リスナーが積み上がって重くなる。
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

    // f キーで全体を画面に収める。
    window.addEventListener("keydown", function (e) {
      if (e.key === "f" && lastLayout) fit(lastLayout);
    });
  }

  // ノード側の操作は描き直しのたびに結び直す。
  function bindNodes(svg) {
    // 集約のクリックで展開／折りたたみ。中身のクリックは伝播させない。
    svg.querySelectorAll(".agg").forEach(function (g) {
      g.addEventListener("click", function (e) {
        if (e.target.closest(".member")) return;
        var name = g.getAttribute("data-agg");
        if (expanded.has(name)) expanded.delete(name);
        else expanded.add(name);
        layout();
      });
    });

    // ホバーで関連する辺と集約を残し、他を沈める。
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

  // ---- 実行 --------------------------------------------------------

  function layout() {
    elk
      .layout(buildElk())
      .then(render)
      .catch(function (err) {
        stage.innerHTML =
          '<pre style="padding:20px;color:var(--accent)">レイアウトに失敗: ' +
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
      '<div class="sub">' + aggCount + " 集約 / " + refCount + " 参照</div>";

    html +=
      '<h2>操作</h2><div class="hint">' +
      "集約をクリックで展開・折りたたみ<br>" +
      "ホバーで関連を強調<br>" +
      "ドラッグで移動、<kbd>wheel</kbd> で拡大縮小<br>" +
      "<kbd>f</kbd> で全体を表示<br><br>" +
      '<button class="link" id="all-open">すべて展開</button> / ' +
      '<button class="link" id="all-close">すべて畳む</button>' +
      "</div>";

    if (graph.unclassified.length) {
      html +=
        "<h2>未分類 " + graph.unclassified.length + "</h2><ul>" +
        graph.unclassified
          .map(function (u) {
            return "<li><b>" + escapeHtml(u.name) + "</b><br>" + escapeHtml(u.pos) + "</li>";
          })
          .join("") +
        "</ul>" +
        '<div class="hint" style="margin-top:8px">どの集約からも到達できない型。' +
        "ドメインサービスや DTO のことが多いが、マーカーの付け忘れもここに出る。</div>";
    }

    side.innerHTML = html;

    // 全体を見る操作なので、描き直したあと画面に収め直す。
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

  function escapeHtml(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
    });
  }

  buildSide();
  bindStage();
  layout();
})();
