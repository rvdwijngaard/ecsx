# Layered TUI Architecture Pattern

A four-layer architecture for terminal UI applications that interact with
external services. Each layer has a single responsibility and dependencies only
flow downward.

## Layer Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Views (Presentation)                            │
│                     UI components, user interaction, rendering               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  View A (e.g. list/selection)           View B (e.g. detail/editor)         │
│  ┌───────────────────────────┐          ┌────────────────────────────────┐  │
│  │ • List resources          │          │ • Display resource details     │  │
│  │ • Search/filter           │  ─────▶  │ • Paginated data fetching      │  │
│  │ • Navigate & select       │  Select  │ • Sorting / filtering          │  │
│  │                           │          │ • Format toggling              │  │
│  │                           │  ◀─────  │ • Session state per resource   │  │
│  └───────────────────────────┘  Back    └────────────────────────────────┘  │
│         │                                         │                          │
│         │ calls adapter functions                  │ calls adapter functions  │
│         ▼                                         ▼                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                         Adapters (Translation layer)                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │ • Translates between UI types and connector types                   │    │
│  │ • Calls connector functions                                         │    │
│  │ • Parses raw responses into display-ready formats                   │    │
│  │ • Produces styled/formatted output for the terminal                 │    │
│  │ • Enriches data with presentation metadata                          │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│         │                                                                    │
│         │ calls connector                                                    │
│         ▼                                                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                        Connectors (Infrastructure layer)                     │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │ • Thin wrapper around external SDK / API client                     │    │
│  │ • Builds request structs from its own domain types                  │    │
│  │ • Executes operations against the external service                  │    │
│  │ • Returns raw, unformatted responses                                │    │
│  │ • Handles authentication, retries, timeouts                         │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                          Messages (Event bus)                                │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │ • Defines all message/event types                                   │    │
│  │ • Shared vocabulary between all views, panes, and dialogs           │    │
│  │ • No logic — pure data structures                                   │    │
│  │ • Enables lateral communication without direct coupling             │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Data Flow (generic)

```
User triggers action in View A
       │
       ▼
View A ──navigation msg──▶ View B
                              │
                              │ initiates data fetch
                              ▼
                        Adapter.Operation()
                              │
                              │ translates types, delegates
                              ▼
                        Connector.Operation()
                              │
                              │ builds request, calls external service
                              ▼
                        External Service (API / DB / filesystem)
                              │
                              │ raw response
                              ▼
                        Connector returns raw domain response
                              │
                              ▼
                        Adapter transforms response
                          → display-ready strings
                          → styled/formatted output
                          → extracted metadata for UI components
                              │
                              ▼
                        View B receives result message
                          → updates UI state
                          → renders to terminal
```

## Layer Responsibilities

| Layer          | Responsibility                                                 | Depends on                                     |
| -------------- | -------------------------------------------------------------- | ---------------------------------------------- |
| **Connectors** | Execute operations against external services, return raw data  | External SDK / API only                        |
| **Adapters**   | Translate between UI and connector types, parse and style data | Connectors, own UI-oriented types              |
| **Messages**   | Define the event contract between all UI components            | Adapter types (for payloads)                   |
| **Views**      | Present data, handle user input, manage UI state               | Adapters (to fetch), Messages (to communicate) |

## Type Ownership

Each layer defines its own request/response types to maintain isolation:

```
┌────────────┐       ┌────────────┐       ┌────────────┐
│  View      │       │  Adapter   │       │ Connector  │
│  types     │ ───▶  │  types     │ ───▶  │  types     │
│            │       │            │       │            │
│ • UI state │       │ • Display  │       │ • Raw req  │
│ • Render   │       │   formats  │       │ • Raw resp │
│   params   │       │ • Styled   │       │ • Domain   │
│            │       │   output   │       │   models   │
└────────────┘       └────────────┘       └────────────┘
```

The adapter layer is responsible for mapping between connector types (raw) and
its own types (display-ready). Views consume adapter types directly.

## Design Principles

1. **Downward-only dependencies** — Views → Adapters → Connectors. Never upward.
2. **Messages for lateral communication** — Views never import each other; they
   communicate through shared message types.
3. **Each layer owns its types** — No layer reuses another layer's
   request/response structs directly. Adapters translate between them.
4. **Connectors are SDK-agnostic to callers** — The adapter shields the UI from
   SDK changes. Swapping the external service only affects the connector and
   adapter.
5. **Views are composable** — Each view is a self-contained component (panes,
   keymaps, state) that can be assembled into larger layouts.
6. **Stateless adapters, stateful views** — Adapters are pure functions (input →
   output). All mutable state lives in views.

## Client Lifecycle & Dependency Injection

Bubble Tea models use value receivers, so reassigning fields inside `Init()` or
`Update()` does not persist. To handle clients that are created or refreshed
after construction (e.g. after loading AWS credentials, switching regions):

- Pass a **shared config pointer** (`*appconfig.Config`) to views at
  construction time.
- Store SDK clients on the config struct (e.g. `config.ECSClient`).
- Views read the client from the config pointer **at call time** (inside the
  command function), not at construction time.
- When the client needs to change (region switch, credential refresh), update
  the config pointer and call `Init()` on the view — no need to recreate it.

This avoids the need to reconstruct the entire view tree when external clients
change, while keeping the dependency flow clean (views depend on config, config
holds clients that are set by the top-level model).

## When to Use This Pattern

- Terminal UI apps (Bubble Tea, tview, etc.) that talk to external services
- Any TUI with multiple "screens" or "views" that share data
- Projects where you want to swap the backing service without rewriting the UI
- Apps that need formatted/styled terminal output derived from raw API data

## File Structure (example)

```
pkg/
├── ui/
│   └── internal/
│       ├── views/
│       │   ├── list/          # View A — resource listing
│       │   └── detail/        # View B — resource detail/editor
│       ├── adapters/
│       │   └── <service>/     # Adapter for a specific service
│       │       ├── adapter.go
│       │       ├── types/
│       │       └── parsing/
│       ├── messages/
│       │   └── messages.go    # All shared message types
│       ├── styles/            # Shared terminal styles
│       └── components/        # Reusable UI components (tables, lists, search)
├── <service>/                 # Connector for external service
│   ├── client.go
│   ├── operations.go
│   └── types/
└── config/                    # Shared app configuration
```
