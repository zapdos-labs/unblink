# Windows Node Setup + Autorun Plan

## Goal
Provide a one-time interactive setup flow, then optionally install WinSW autorun for non-technical users.

## User Flow
1. User runs `node.exe` interactively once.
2. Node performs token/bootstrap and registration.
3. Node prints node/dashboard info clearly.
4. Node asks: `Enable auto-start on boot? (y/N)`.
5. If `yes`, node installs and starts WinSW service.

## Implementation Scope
- Add a setup mode flag, e.g. `node setup` (or `-setup`).
- Keep normal `node` runtime behavior unchanged.
- Add Windows-only service installer helpers:
  - Validate running as Administrator.
  - Create/update WinSW XML with absolute paths.
  - Run `unblink-node-service.exe install` then `start`.
  - Handle already-installed service idempotently.
- Persist and show:
  - config path
  - node ID
  - dashboard URL
  - service log path

## Packaging Expectations
- Ship these files together:
  - `node.exe`
  - `unblink-node-service.exe` (WinSW)
  - `unblink-node-service.xml` (template or generated)
  - optional `install-service.bat` / `uninstall-service.bat`
- Use stable install/config/log locations (absolute paths).

## Error Handling
- If not elevated: print exact steps to rerun as Administrator.
- If service install/start fails: print command, exit code, and log location.
- If setup succeeds but autorun fails: node remains usable in manual mode.

## Test Strategy (No Local Windows)
- Add CI on `windows-latest`:
  - bootstrap config
  - install service
  - start/status/stop/uninstall
  - assert expected files/logs exist
- Keep service logic behind small interface to allow unit tests on non-Windows.
