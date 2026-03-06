// === Battleship Client ===

(function () {
  "use strict";

  // --- State ---
  let ws = null;
  let state = {
    playerID: "",
    roomCode: "",
    gameID: "",
    mode: "", // "ai" or "human"
    config: null,
    ships: [],
    phase: "menu", // menu, waiting, placement, firing, gameover

    // Placement state
    orientation: 0, // 0 = horizontal, 1 = vertical
    selectedShip: null,
    placedShips: {}, // shipName -> { start, orientation, length }
    remainingShips: [],

    // Firing state
    myTurn: false,
    ownBoard: null, // 2D array of cell states
    opponentBoard: null,
    sunkOpponentShips: [],
    sunkOwnShips: [],
  };

  // --- DOM refs ---
  const screens = {
    menu: document.getElementById("screen-menu"),
    waiting: document.getElementById("screen-waiting"),
    placement: document.getElementById("screen-placement"),
    firing: document.getElementById("screen-firing"),
    gameover: document.getElementById("screen-gameover"),
  };

  // --- Helpers ---
  function showScreen(name) {
    Object.values(screens).forEach((s) => s.classList.remove("active"));
    screens[name].classList.add("active");
    state.phase = name;
  }

  function send(msg) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg));
    }
  }

  function colLabel(x) {
    // A-Z for 0-25, then AA, AB, etc. for 26+
    let label = "";
    let n = x;
    do {
      label = String.fromCharCode(65 + (n % 26)) + label;
      n = Math.floor(n / 26) - 1;
    } while (n >= 0);
    return label;
  }

  function log(text, cls) {
    const el = document.getElementById("game-log");
    const entry = document.createElement("div");
    entry.className = "log-entry" + (cls ? " log-" + cls : "");
    entry.textContent = text;
    el.prepend(entry);
  }

  // --- Session Persistence (survive page refresh) ---
  function saveSession() {
    try {
      sessionStorage.setItem(
        "battleship_session",
        JSON.stringify({
          playerID: state.playerID,
          roomCode: state.roomCode,
          gameID: state.gameID,
          mode: state.mode,
        })
      );
    } catch (e) {
      // sessionStorage may be unavailable
    }
  }

  function clearSession() {
    try {
      sessionStorage.removeItem("battleship_session");
    } catch (e) {
      // ignore
    }
  }

  function tryRejoinFromSession() {
    try {
      const raw = sessionStorage.getItem("battleship_session");
      if (!raw) return false;
      const session = JSON.parse(raw);
      if (session.playerID && session.roomCode) {
        state.playerID = session.playerID;
        state.roomCode = session.roomCode;
        state.gameID = session.gameID;
        state.mode = session.mode || "human";
        connect(() => {
          send({
            type: "rejoin_game",
            room_code: state.roomCode,
            player_id: state.playerID,
          });
        });
        return true;
      }
    } catch (e) {
      // ignore parse errors
    }
    return false;
  }

  // --- WebSocket ---
  let pendingOnOpen = null; // callback to run once WS opens

  function connect(onOpen) {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    ws = new WebSocket(`${proto}//${location.host}/ws`);
    pendingOnOpen = onOpen || null;

    ws.onopen = () => {
      console.log("WS connected");
      if (pendingOnOpen) {
        pendingOnOpen();
        pendingOnOpen = null;
      }
    };
    ws.onclose = () => {
      console.log("WS disconnected");
      // Reconnect after a short delay if mid-game, sending rejoin
      if (
        state.phase !== "menu" &&
        state.phase !== "gameover" &&
        state.roomCode &&
        state.playerID
      ) {
        setTimeout(() => {
          connect(() => {
            send({
              type: "rejoin_game",
              room_code: state.roomCode,
              player_id: state.playerID,
            });
          });
        }, 2000);
      }
    };
    ws.onerror = (e) => console.error("WS error:", e);
    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data);
      handleMessage(msg);
    };
  }

  function handleMessage(msg) {
    switch (msg.type) {
      case "error":
        console.error("Server error:", msg.message);
        alert(msg.message);
        break;

      case "game_created":
        state.playerID = msg.player_id;
        state.roomCode = msg.room_code;
        state.gameID = msg.game_id;
        state.config = msg.config;
        state.ships = msg.ships;
        saveSession();
        if (state.mode === "human") {
          document.getElementById("display-room-code").textContent =
            msg.room_code;
          showScreen("waiting");
        }
        // For AI, we wait for game_start
        break;

      case "game_joined":
        state.playerID = msg.player_id;
        state.roomCode = msg.room_code;
        state.gameID = msg.game_id;
        state.config = msg.config;
        state.ships = msg.ships;
        saveSession();
        // Wait for game_start
        break;

      case "game_start":
        startPlacement();
        break;

      case "place_result":
        onPlaceResult(msg);
        break;

      case "opponent_ready":
        document.getElementById("placement-status").textContent =
          "Opponent is ready!";
        break;

      case "all_placed":
        startFiring(msg.your_turn);
        break;

      case "fire_result":
        onFireResult(msg);
        break;

      case "opponent_fired":
        onOpponentFired(msg);
        break;

      case "turn_update":
        setTurn(msg.your_turn);
        break;

      case "game_over":
        onGameOver(msg);
        break;

      case "game_state":
        onGameState(msg);
        break;

      case "opponent_left":
        if (state.phase !== "gameover") {
          log("Opponent disconnected.", "miss");
        }
        break;
    }
  }

  // --- Menu ---
  document.getElementById("btn-vs-ai").addEventListener("click", () => {
    state.mode = "ai";
    connect(() => {
      send({ type: "create_game", mode: "ai" });
    });
  });

  document.getElementById("btn-vs-human").addEventListener("click", () => {
    state.mode = "human";
    connect(() => {
      send({ type: "create_game", mode: "human" });
    });
  });

  document.getElementById("btn-join").addEventListener("click", () => {
    const code = document.getElementById("input-room-code").value.trim();
    if (!code) return;
    state.mode = "human";
    connect(() => {
      send({ type: "join_game", room_code: code.toUpperCase() });
    });
  });

  // Allow Enter key on room code input
  document
    .getElementById("input-room-code")
    .addEventListener("keydown", (e) => {
      if (e.key === "Enter") document.getElementById("btn-join").click();
    });

  document.getElementById("btn-rematch").addEventListener("click", () => {
    const mode = state.mode || "ai";
    resetState();
    state.mode = mode;
    connect(() => {
      send({ type: "create_game", mode: mode });
    });
  });

  document.getElementById("btn-menu").addEventListener("click", () => {
    resetState();
    showScreen("menu");
  });

  function resetState() {
    if (ws) ws.close();
    clearSession();
    state = {
      playerID: "",
      roomCode: "",
      gameID: "",
      mode: state.mode,
      config: null,
      ships: [],
      phase: "menu",
      orientation: 0,
      selectedShip: null,
      placedShips: {},
      remainingShips: [],
      myTurn: false,
      ownBoard: null,
      opponentBoard: null,
      sunkOpponentShips: [],
      sunkOwnShips: [],
    };
    document.getElementById("game-log").innerHTML = "";
  }

  // --- Board Rendering ---
  function createBoard(container, size, clickHandler) {
    container.innerHTML = "";
    // +1 for headers
    container.style.gridTemplateColumns = `repeat(${size + 1}, var(--cell-size))`;
    container.style.gridTemplateRows = `repeat(${size + 1}, var(--cell-size))`;

    // Corner
    const corner = document.createElement("div");
    corner.className = "cell-header cell-corner";
    container.appendChild(corner);

    // Column headers (A-Z, then AA, AB, etc.)
    for (let x = 0; x < size; x++) {
      const header = document.createElement("div");
      header.className = "cell-header";
      header.textContent = colLabel(x);
      container.appendChild(header);
    }

    // Rows
    for (let y = 0; y < size; y++) {
      // Row header
      const rowHeader = document.createElement("div");
      rowHeader.className = "cell-header";
      rowHeader.textContent = y + 1;
      container.appendChild(rowHeader);

      for (let x = 0; x < size; x++) {
        const cell = document.createElement("div");
        cell.className = "cell";
        cell.dataset.x = x;
        cell.dataset.y = y;
        if (clickHandler) {
          cell.addEventListener("click", () => clickHandler(x, y, cell));
        }
        container.appendChild(cell);
      }
    }
  }

  function getCell(container, x, y) {
    return container.querySelector(`[data-x="${x}"][data-y="${y}"]`);
  }

  // --- Placement ---
  function startPlacement() {
    const size = state.config.board_size;
    state.remainingShips = [...state.ships];
    state.placedShips = {};
    state.orientation = 0;

    // Init own board state
    state.ownBoard = Array.from({ length: size }, () =>
      Array(size).fill("empty")
    );

    showScreen("placement");
    renderShipList();
    createBoard(
      document.getElementById("board-placement"),
      size,
      onPlacementClick
    );

    // Select first ship
    if (state.remainingShips.length > 0) {
      selectShip(state.remainingShips[0].name);
    }

    // Setup hover preview
    const board = document.getElementById("board-placement");
    board.addEventListener("mouseover", onPlacementHover);
    board.addEventListener("mouseout", clearPreview);
  }

  function renderShipList() {
    const list = document.getElementById("ship-list");
    list.innerHTML = "";

    state.ships.forEach((ship) => {
      const item = document.createElement("div");
      item.className = "ship-item";
      if (state.placedShips[ship.name]) item.classList.add("placed");
      if (state.selectedShip === ship.name) item.classList.add("selected");

      const nameSpan = document.createElement("span");
      nameSpan.textContent = `${ship.name} (${ship.length})`;

      const dots = document.createElement("div");
      dots.className = "ship-dots";
      for (let i = 0; i < ship.length; i++) {
        const dot = document.createElement("div");
        dot.className = "ship-dot";
        dots.appendChild(dot);
      }

      item.appendChild(nameSpan);
      item.appendChild(dots);

      if (!state.placedShips[ship.name]) {
        item.addEventListener("click", () => selectShip(ship.name));
      }

      list.appendChild(item);
    });

    const remaining = state.ships.filter((s) => !state.placedShips[s.name]);
    document.getElementById("placement-status").textContent =
      remaining.length > 0
        ? `${remaining.length} ship(s) remaining`
        : "All ships placed!";
  }

  function selectShip(name) {
    if (state.placedShips[name]) return;
    state.selectedShip = name;
    renderShipList();
  }

  function getShipConfig(name) {
    return state.ships.find((s) => s.name === name);
  }

  function getShipCells(x, y, length, orientation) {
    const cells = [];
    for (let i = 0; i < length; i++) {
      if (orientation === 0) {
        cells.push({ x: x + i, y });
      } else {
        cells.push({ x, y: y + i });
      }
    }
    return cells;
  }

  function isValidPlacement(x, y, length, orientation) {
    const size = state.config.board_size;
    const cells = getShipCells(x, y, length, orientation);
    for (const c of cells) {
      if (c.x < 0 || c.x >= size || c.y < 0 || c.y >= size) return false;
      if (state.ownBoard[c.y][c.x] === "ship") return false;
    }
    return true;
  }

  function onPlacementHover(e) {
    const cell = e.target.closest(".cell");
    if (!cell || !state.selectedShip) return;
    clearPreview();

    const x = parseInt(cell.dataset.x);
    const y = parseInt(cell.dataset.y);
    const ship = getShipConfig(state.selectedShip);
    if (!ship) return;

    const cells = getShipCells(x, y, ship.length, state.orientation);
    const valid = isValidPlacement(x, y, ship.length, state.orientation);

    const board = document.getElementById("board-placement");
    cells.forEach((c) => {
      const el = getCell(board, c.x, c.y);
      if (el) {
        el.classList.add(valid ? "preview-valid" : "preview-invalid");
      }
    });
  }

  function clearPreview() {
    document.querySelectorAll(".preview-valid, .preview-invalid").forEach((c) => {
      c.classList.remove("preview-valid", "preview-invalid");
    });
  }

  function onPlacementClick(x, y) {
    if (!state.selectedShip) return;
    const ship = getShipConfig(state.selectedShip);
    if (!ship) return;

    if (!isValidPlacement(x, y, ship.length, state.orientation)) return;

    send({
      type: "place_ship",
      ship_name: state.selectedShip,
      start: { x, y },
      orientation: state.orientation,
    });
  }

  function onPlaceResult(msg) {
    const ship = getShipConfig(msg.ship_name);
    if (!ship) return;

    const orient = msg.orientation;
    const start = msg.start;
    state.placedShips[msg.ship_name] = {
      start,
      orientation: orient,
      length: ship.length,
    };

    // Update own board
    const cells = getShipCells(start.x, start.y, ship.length, orient);
    const board = document.getElementById("board-placement");
    cells.forEach((c) => {
      state.ownBoard[c.y][c.x] = "ship";
      const el = getCell(board, c.x, c.y);
      if (el) el.className = "cell ship";
    });

    // Select next unplaced ship
    state.selectedShip = null;
    const next = state.ships.find((s) => !state.placedShips[s.name]);
    if (next) selectShip(next.name);

    renderShipList();
  }

  // Rotate
  document.getElementById("btn-rotate").addEventListener("click", () => {
    state.orientation = state.orientation === 0 ? 1 : 0;
    clearPreview();
  });

  document.addEventListener("keydown", (e) => {
    if (e.key === "r" || e.key === "R") {
      if (state.phase === "placement") {
        state.orientation = state.orientation === 0 ? 1 : 0;
        clearPreview();
      }
    }
  });

  // --- Firing ---
  function startFiring(yourTurn) {
    const size = state.config.board_size;

    // Init opponent board state (all unknown)
    state.opponentBoard = Array.from({ length: size }, () =>
      Array(size).fill("unknown")
    );

    showScreen("firing");

    // Render own board (showing ships and incoming hits)
    createBoard(document.getElementById("board-own"), size, null);
    renderOwnBoard();

    // Render opponent board (clickable)
    createBoard(
      document.getElementById("board-opponent"),
      size,
      onFireClick
    );
    renderOpponentBoard();

    setTurn(yourTurn);
  }

  function renderOwnBoard() {
    const board = document.getElementById("board-own");
    const size = state.config.board_size;

    for (let y = 0; y < size; y++) {
      for (let x = 0; x < size; x++) {
        const cell = getCell(board, x, y);
        if (!cell) continue;
        cell.className = "cell";
        const s = state.ownBoard[y][x];
        if (s === "ship") cell.classList.add("ship");
        else if (s === "hit") cell.classList.add("hit");
        else if (s === "miss") cell.classList.add("miss");
        else if (s === "sunk") cell.classList.add("sunk");
      }
    }
  }

  function renderOpponentBoard() {
    const board = document.getElementById("board-opponent");
    const size = state.config.board_size;

    for (let y = 0; y < size; y++) {
      for (let x = 0; x < size; x++) {
        const cell = getCell(board, x, y);
        if (!cell) continue;
        cell.className = "cell";
        const s = state.opponentBoard[y][x];
        if (s === "hit") cell.classList.add("hit");
        else if (s === "miss") cell.classList.add("miss");
        else if (s === "sunk") cell.classList.add("sunk");
        else cell.classList.add("unknown");
      }
    }
  }

  function setTurn(yourTurn) {
    state.myTurn = yourTurn;
    const indicator = document.getElementById("turn-indicator");
    const opBoard = document.getElementById("board-opponent");

    if (yourTurn) {
      indicator.textContent = "Your Turn — Fire!";
      indicator.className = "turn-indicator your-turn";
      opBoard.classList.remove("disabled");
    } else {
      indicator.textContent = "Opponent's Turn...";
      indicator.className = "turn-indicator opponent-turn";
      opBoard.classList.add("disabled");
    }
  }

  function onFireClick(x, y) {
    if (!state.myTurn) return;
    if (state.opponentBoard[y][x] !== "unknown") return;

    send({ type: "fire", target: { x, y } });
    state.myTurn = false; // Optimistic: disable until server confirms
    document.getElementById("board-opponent").classList.add("disabled");
  }

  function onFireResult(msg) {
    const { x, y } = msg.coord;
    const colLetter = colLabel(x);
    const rowNum = y + 1;

    if (msg.hit) {
      if (msg.sunk_ship) {
        state.sunkOpponentShips.push(msg.sunk_ship);
        // Use server-provided coordinates to mark all cells of the sunk ship.
        markSunkCoordsOnBoard(state.opponentBoard, msg.sunk_ship_coords, x, y);
        log(`You sunk their ${msg.sunk_ship}!`, "sunk");
      } else {
        state.opponentBoard[y][x] = "hit";
        log(`${colLetter}${rowNum} — Hit!`, "hit");
      }
    } else {
      state.opponentBoard[y][x] = "miss";
      log(`${colLetter}${rowNum} — Miss`, "miss");
    }

    renderOpponentBoard();
  }

  function onOpponentFired(msg) {
    const { x, y } = msg.coord;
    const colLetter = colLabel(x);
    const rowNum = y + 1;

    if (msg.hit) {
      if (msg.sunk_ship) {
        state.sunkOwnShips.push(msg.sunk_ship);
        markSunkCoordsOnBoard(state.ownBoard, msg.sunk_ship_coords, x, y);
        log(`They sunk your ${msg.sunk_ship}!`, "sunk");
      } else {
        state.ownBoard[y][x] = "hit";
        log(`Opponent hit ${colLetter}${rowNum}`, "hit");
      }
    } else {
      state.ownBoard[y][x] = "miss";
      log(`Opponent missed ${colLetter}${rowNum}`, "miss");
    }

    renderOwnBoard();
  }

  // Mark sunk ship cells using server-provided coordinates (accurate).
  // Falls back to BFS if coordinates not provided (backward compat).
  function markSunkCoordsOnBoard(board, coords, fallbackX, fallbackY) {
    if (coords && coords.length > 0) {
      for (const c of coords) {
        board[c.y][c.x] = "sunk";
      }
    } else {
      // Fallback: BFS from the sunk cell.
      markSunkCellsBFS(board, fallbackX, fallbackY);
    }
  }

  // Legacy BFS fallback for marking sunk cells.
  function markSunkCellsBFS(board, startX, startY) {
    const size = state.config.board_size;
    const visited = new Set();
    const queue = [{ x: startX, y: startY }];
    visited.add(`${startX},${startY}`);

    while (queue.length > 0) {
      const { x, y } = queue.shift();
      board[y][x] = "sunk";

      const neighbors = [
        { x: x - 1, y },
        { x: x + 1, y },
        { x, y: y - 1 },
        { x, y: y + 1 },
      ];

      for (const n of neighbors) {
        const key = `${n.x},${n.y}`;
        if (
          n.x >= 0 && n.x < size &&
          n.y >= 0 && n.y < size &&
          !visited.has(key) &&
          board[n.y][n.x] === "hit"
        ) {
          visited.add(key);
          queue.push(n);
        }
      }
    }
  }

  // --- Reconnection State Hydration ---
  function onGameState(msg) {
    state.playerID = msg.player_id;
    state.roomCode = msg.room_code;
    state.gameID = msg.game_id;
    state.config = msg.config;
    state.ships = msg.ships || [];

    const size = state.config.board_size;

    if (msg.phase === "placement") {
      // Restore placement state.
      state.ownBoard = msg.own_board || Array.from({ length: size }, () => Array(size).fill("empty"));
      state.placedShips = {};
      if (msg.placed_ships) {
        for (const ps of msg.placed_ships) {
          state.placedShips[ps.name] = {
            start: ps.start,
            orientation: ps.orientation,
            length: ps.length,
          };
        }
      }
      state.remainingShips = state.ships.filter((s) => !state.placedShips[s.name]);
      state.orientation = 0;

      showScreen("placement");
      renderShipList();
      createBoard(
        document.getElementById("board-placement"),
        size,
        onPlacementClick
      );

      // Render already-placed ships on the board.
      const board = document.getElementById("board-placement");
      for (let y = 0; y < size; y++) {
        for (let x = 0; x < size; x++) {
          if (state.ownBoard[y][x] === "ship") {
            const el = getCell(board, x, y);
            if (el) el.className = "cell ship";
          }
        }
      }

      // Select next unplaced ship.
      const next = state.ships.find((s) => !state.placedShips[s.name]);
      if (next) selectShip(next.name);

      board.addEventListener("mouseover", onPlacementHover);
      board.addEventListener("mouseout", clearPreview);

      log("Reconnected. Continue placing ships.", "hit");

    } else if (msg.phase === "firing") {
      // Restore firing state.
      state.ownBoard = msg.own_board || Array.from({ length: size }, () => Array(size).fill("empty"));
      state.opponentBoard = msg.opponent_board || Array.from({ length: size }, () => Array(size).fill("unknown"));
      state.placedShips = {};
      if (msg.placed_ships) {
        for (const ps of msg.placed_ships) {
          state.placedShips[ps.name] = {
            start: ps.start,
            orientation: ps.orientation,
            length: ps.length,
          };
        }
      }

      showScreen("firing");
      createBoard(document.getElementById("board-own"), size, null);
      renderOwnBoard();
      createBoard(
        document.getElementById("board-opponent"),
        size,
        onFireClick
      );
      renderOpponentBoard();
      setTurn(msg.your_turn);

      log("Reconnected. Game resumed.", "hit");

    } else if (msg.phase === "finished") {
      // Game already over.
      const youWin = msg.winner === state.playerID;
      onGameOver({ you_win: youWin, winner: msg.winner });
    } else {
      // Waiting phase — show waiting screen.
      document.getElementById("display-room-code").textContent = msg.room_code;
      showScreen("waiting");
      log("Reconnected. Waiting for opponent.", "hit");
    }
  }

  // --- Game Over ---
  function onGameOver(msg) {
    clearSession();
    const title = document.getElementById("gameover-title");
    const message = document.getElementById("gameover-message");

    if (msg.you_win) {
      title.textContent = "Victory!";
      title.style.color = "#00cc66";
      message.textContent = "You sank all enemy ships!";
    } else {
      title.textContent = "Defeat";
      title.style.color = "var(--hit)";
      message.textContent = "All your ships have been sunk.";
    }

    showScreen("gameover");
  }
  // --- Auto-rejoin on page load ---
  tryRejoinFromSession();
})();
