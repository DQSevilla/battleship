# Sentience Engineering Work Trial

## Overview

Congrats on making it this far. We are excited about the potential to work with you and want to share this project as a way to evaluate your skills and abilities.

## Philosophy

Use all the AI you want. In fact, we will strongly evaluate your ability to build with AI. One of the deliverables, in addition to the project and code, is a writeup on how you approached the problem and how you used AI. Claude Code, Cursor, etc. is encouraged and basically required.

## What We Are Looking For

Two things: ****

1. **Feature Complete Version.** Deliver on all of the requirements.
2. **A Spike.** Equally or more importantly, we want you to show off a spike. A spike is something you are uniquely excited, passionate, and skilled about.

## Deliverables

Please reply to the email thread you received this on with:

1. A web link to a deployed version of the project.
2. A link to the GitHub repo of the code you write.
3. In the repo, include a markdown file writeup about your approach to the problem, how you built it, and any considerations. We will review this together live, along with some additional questions.

---

# Project - Build Battleship

## Rules

Battleship is a 2-player grid-based game. Each player has their own 10×10 grid (hidden from opponent).

- **Placement phase:** Players place ships of varying lengths (Carrier-5, Battleship-4, Cruiser-3, Submarine-3, Destroyer-2). Ships can be placed anywhere on the grid as long as they don't overlap or extend beyond boundaries.
- **Firing phase:** Players take turns picking coordinates to fire at the opponent's grid. Each shot returns "hit" or "miss." When all squares of a ship are hit, it's "sunk" (announce which ship).
- **Win condition:** First player to sink all opponent's ships wins.

## Requirements

### Core Gameplay

- A complete, rules-correct implementation of Battleship with a functional, interactive web frontend.
- Ship placement phase with the ability to rotate and validate positions before confirming.
- Firing phase that clearly displays both your fleet (with incoming hits) and your shots (hits/misses on the opponent's grid), with hit/miss/sunk feedback after every shot.
- Win detection with the option to rematch or return to menu.

### Game Modes

1. **vs. AI (single-player)** — AI ships placed randomly. AI shot logic should be at least moderately intelligent (e.g., probing adjacent cells after a hit, not purely random).
2. **vs. Human (multiplayer)** — Two players in separate browser windows play against each other in real time. Both players must see updates without manually refreshing.

### Hosting

- The game must be deployed to a publicly accessible URL. Include the live link in your submission.

### Persistence

- Game state should survive a page refresh mid-game (at minimum for multiplayer).
- Store completed game history (moves, outcome, timestamps) so it could be queried later. Choose whatever storage layer you think is appropriate and explain your choice.

### Considerations

- How can a player cheat? How can you prevent cheating?
- Runtime complexity - what if the game was scaled up to a huge board?
