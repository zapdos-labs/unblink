This is a monorepo.

Refer to Makefile if you have any doubt.

Run this for catching syntax errors (really fast, for both go and Typescript)

```
make typecheck
```

# Proto
Run `make proto` to rebuild protobuf files.

# Monorepo

You must use `cd`. Your tool call should look like this:
Bash(command="cd $ROOT/app && npx tsc --skipLibCheck 2>&1")

If you run npx in the root, it will give errors because there is no tsconfig.json there.

Backend and frontend talk using Connect RPC. Always use that. use `make proto` to generate proto files.

# Never start the app yourself

If you need testing, always ask user to start it themselves. Do not use `go run` or `bun dev`.

# NixOS
Use `nix.flake` to manage dev deps. 

# Ask for permission before giving up
Do not automatically simplify or fallback. Always ask user "Can I skip this task?" or "Can I simplify?"

# Ask user about migration
Since the app is in beta, migration is not usually needed, and we don't want to pollute the codebase (prefer clean, breaking changes instead). Therefore ask the user for permission before implementing migration / fallback.