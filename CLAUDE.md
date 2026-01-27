This is a monorepo.

Refer to Makefile if you have any doubt.

For catching syntax errors:

```
cd full_path_to_app && npx tsc --skipLibCheck
cd full_path_to_server && go vet ./...
```

# Important

You must use `cd`. Your tool call should look like this:
Bash(command="cd $ROOT/app && npx tsc --skipLibCheck 2>&1")

If you run npx in the root, it will give errors because there is no tsconfig.json there.
