# Project Guidelines

## Architecture

This project follows the **Layered TUI Architecture Pattern** defined in [architecture-pattern.md](./architecture-pattern.md).

All new code must conform to this pattern:

- **Views** — presentation layer, UI components, user interaction
- **Adapters** — translation layer between UI types and connector types, formatting/styling
- **Connectors** — thin wrappers around external services, return raw data
- **Messages** — shared event/message types for lateral communication between views

Key rules:
1. Dependencies flow downward only: Views → Adapters → Connectors
2. Views communicate laterally through Messages, never by importing each other
3. Each layer owns its own types — no reusing another layer's structs directly
4. Adapters are stateless pure functions; all mutable state lives in Views
5. Connectors are SDK-agnostic to callers — adapter shields UI from SDK changes

When adding new features, follow the file structure in architecture-pattern.md.
