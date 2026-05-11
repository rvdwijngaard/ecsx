# Architecture Comparison: ecsx vs dynamite

## Overview

| Aspect | ecsx | dynamite |
|--------|------|----------|
| Domain | Amazon ECS (clusters, services, tasks, logs) | Amazon DynamoDB (tables, items, queries) |
| Go version | 1.25 | 1.26.2 |
| CLI framework | spf13/cobra | urfave/cli/v3 |
| TUI framework | bubbletea v2 + bubbles v2 + lipgloss v2 | bubbletea v2 + bubbles v2 + lipgloss v2 |
| Architecture | Flat monolithic model | Layered: views → panes → components → dialogs |
| Config | CLI flags only | CLI flags + YAML config file |
| AWS auth | Profile/region via flags/env | Profile/region via flags/env/config + MFA token provider |

## Structural Comparison

### ecsx — Flat Model

```
cmd/ecsx/main.go          ← cobra commands, entry point
internal/
  ui/
    app.go                ← single Model struct, all state in one place
    handlers.go           ← action handlers (enter, esc, scale, deploy, etc.)
    commands.go           ← tea.Cmd factories (load clusters, services, tasks)
    render.go             ← View() helpers
    items.go              ← list item types
    styles.go             ← lipgloss styles
    sparkline.go          ← metrics sparkline widget
    clipboard.go          ← clipboard helper
    editor.go             ← $EDITOR integration
    ssm.go                ← SSM session launcher
  aws/
    client.go             ← ECSClient interface + Client implementation
    cache.go              ← CachedClient decorator
    metrics.go            ← CloudWatch metrics
  logs/                   ← log tailing, filtering, grep
  exec/                   ← ECS exec
  ssm/                    ← SSM connect
  debug/                  ← debug logging
```

The UI is a **single `Model`** with all state (view level, log state, scaling state, env vars, etc.) managed via fields and a large `Update()` switch. Navigation is a `viewLevel` enum. There are no sub-models for views — the one model handles everything.

### dynamite — Layered Composition

```
cmd/dynamite/main.go      ← urfave/cli entry point
pkg/
  appconfig.go            ← shared Config struct
  configfile/             ← YAML config loading
  aws/
    awsconfig.go          ← AWS config loader with MFA callback
    dynamodb/
      client.go           ← DynamoDB client wrapper
      keys.go             ← key schema helpers
      table.go            ← table operations
      types/              ← domain types
  ui/
    home.go               ← root Model, orchestrates views + dialogs
    keymap.go             ← top-level keybindings
    internal/
      messages/           ← shared message types (decoupled communication)
      views/
        tables/           ← TableSelection view (2-pane: list + details)
          view.go         ← view coordinator with focus management
          paneTables.go   ← left pane (table list)
          paneDetails.go  ← right pane (table details)
          keymap.go       ← view-specific keybindings
          help.go         ← view-specific help
        items/            ← ItemSelection view (2-pane: items + details)
          view.go         ← view coordinator
          paneItems.go    ← left pane (item list)
          paneDetails.go  ← right pane (item detail)
          scan.go         ← scan logic
          query.go        ← query logic
          keymap.go       ← view-specific keybindings
          help.go         ← view-specific help
      dialogs/            ← modal dialogs (help, regions, columns, sort, scan, query, copy, mfa)
      components/         ← reusable UI components (table, search, lists)
      adapters/           ← DynamoDB response parsing (JSON/YAML)
      styles/             ← shared style definitions
  util/                   ← generic helpers
```

The UI uses a **hierarchical composition** pattern:
- **Root Model** → routes messages to the active **View**
- **Views** (tables, items) → each is a 2-pane coordinator that routes to focused **Pane**
- **Panes** → self-contained bubbletea sub-models
- **Dialogs** → modal overlays managed by root, rendered via lipgloss layers
- **Messages** → a dedicated package for inter-component communication

## Key Architectural Differences

### 1. Message Routing

| | ecsx | dynamite |
|---|------|----------|
| Strategy | Single switch in one `Update()` | Hierarchical: broadcast non-key msgs, route key msgs to focused child |
| Coupling | All handlers know all state | Views/panes only know their own state |
| Scalability | Adding features grows the switch | Adding features means adding a new view/pane/dialog |

Dynamite's `forward()` pattern:
- Non-key messages → **broadcast** to all children (so everyone can react to data changes)
- Key presses → **route to active only** (dialogs take precedence over views, focused pane takes precedence)

### 2. State Management

**ecsx**: One flat struct with ~40 fields. View level tracked by an enum. Modal states (scaling, confirming, filtering, grepping) are booleans that gate key handling.

**dynamite**: State is distributed across the component tree. Each view/pane owns its own state. The root model only tracks which view is active and which dialog is open.

### 3. Dialog System

**ecsx**: No formal dialog system. Modal states (scale input, confirm, help) are inline in the model with boolean flags.

**dynamite**: Dedicated `dialogs/` package. Each dialog is a self-contained model. The root manages open/close via toggle methods and renders dialogs as lipgloss layers (compositor) centered on screen.

### 4. Keybinding Architecture

**ecsx**: Keybindings are string-matched in the `Update()` switch (`msg.String() == "s"`).

**dynamite**: Uses `bubbles/key` bindings with named `KeyMap` structs at every level. Views declare their own keymaps and expose them for the help dialog. Parent keymaps are injected into children via `AdditionalKeys`.

### 5. AWS Client Layer

**ecsx**: `ECSClient` interface with a concrete `Client` and a `CachedClient` decorator. Clean separation — the UI depends on the interface.

**dynamite**: Direct `*dynamodb.Client` stored in the shared config. No interface abstraction (harder to test/mock).

### 6. Configuration

**ecsx**: Flags only. No persistent config.

**dynamite**: YAML config file (`~/.config/dynamite/config.yaml`) with defaults for profile, region, starred regions, max tables. Loaded at startup with error handling.

## Recommendations: Aligning ecsx with dynamite's Architecture

### Phase 1 — Extract Message Types

Create `internal/ui/messages/messages.go` with typed messages instead of anonymous structs scattered through the code. This decouples components.

```
internal/ui/messages/
  messages.go    ← LoadClusters, LoadServices, SwitchView, ToggleHelp, etc.
```

### Phase 2 — Extract Views as Sub-Models

Split the monolithic `Model` into view-specific sub-models:

```
internal/ui/
  home.go              ← root model (view routing, dialog management)
  views/
    clusters/view.go   ← cluster list + detail pane
    services/view.go   ← service list + detail pane (metrics, env vars)
    tasks/view.go      ← task list + detail pane
    logs/view.go       ← log tailing view
```

Each view implements its own `Update()` and `View()`. The root model routes messages using the broadcast/route-to-active pattern from dynamite.

### Phase 3 — Introduce a Dialog System

Extract help, scale-input, and confirm prompts into a `dialogs/` package with a consistent open/close lifecycle. Render them as lipgloss layers.

### Phase 4 — Formalize Keybindings

Replace string matching with `bubbles/key` KeyMap structs. Each view declares its own keymap. This enables:
- Context-sensitive help (like dynamite's help dialog showing bindings per view)
- Disabling bindings when a dialog is open

### Phase 5 — Add Config File Support (optional)

Add a YAML config file for defaults (profile, region, preferred cluster, theme colors). Follow dynamite's `configfile` pattern.

## What ecsx Already Does Well (Keep These)

- **ECSClient interface** — better than dynamite's direct client reference. Keep this for testability.
- **CachedClient decorator** — smart pattern for reducing API calls during auto-refresh.
- **CLI subcommands** (logs, exec, ssm, task, container-env) — dynamite is TUI-only; ecsx's CLI commands are a strength.
- **Auto-refresh** — dynamite doesn't have this.
- **Log tailing with grep/level filtering** — unique to ecsx, well-implemented.

## Summary

The main architectural gap is that ecsx puts everything in one model while dynamite distributes responsibility across a view/pane/dialog hierarchy with message-based communication. Adopting dynamite's layered approach would make ecsx easier to extend (new views, new dialogs) without growing the central switch statement. The migration can be done incrementally — start with message types, then extract one view at a time.
