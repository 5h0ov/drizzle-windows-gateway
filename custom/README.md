# Custom Enhancements Directory

This directory holds all custom modifications and extensions built on top of Drizzle Gateway without altering upstream extracted code.

## Structure:
- `assets/`: Custom UI overrides, stylesheets, or icons. Any file placed here with the same name as an upstream asset will override it during enhanced builds.
- `plugins/`: Custom server plugins, endpoints (e.g. CSV exporter, schema visualizer, AI assistant).
- `patches/`: Git diff patches to automatically apply to server code.

## Build:
- Vanilla: `bun run scripts/update-gateway.mjs --flavor vanilla` (pure upstream)
- Enhanced: `bun run scripts/update-gateway.mjs --flavor enhanced` (with custom overrides)
