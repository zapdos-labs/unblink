# Unblink Administration Guide

The administration script (`admin.ts`) allows you to perform administrative tasks such as managing users, settings, and secrets directly from the command line.

## Usage

All administration commands are run using the following format:

```bash
# Or "unblink admin" if you are running binary file
bun run admin.ts <command> [subcommand] [arguments]
```

To see a list of all available commands, you can run:

```bash
bun run admin.ts help
```

---

## Enable Authentication Screen

By default, the authentication screen is disabled, and any visitor to the application is granted admin privileges. This is sufficient if you host Unblink on a private network with restricted access.

To enable the authentication screen, modify the settings as follows:

```bash
bun run admin.ts settings modify auth_screen_enabled true
bun run admin.ts users add
# input username, password, and role (admin)
```

Unblink uses role-based access control.

---

## Commands

### User Management (`users`)

The `users` command is used to manage user accounts.

#### `users add`

This command allows you to add a new user. You will be prompted to enter a username, password, and role (`admin` or `viewer`).

**Usage:**
```bash
bun run admin.ts users add
```

---

#### `users update`

This command updates the password for an existing user. You will be prompted to enter the username and the new password.

**Usage:**
```bash
bun run admin.ts users update
```

---

#### `users delete`

This command deletes a user from the system. You will be prompted to enter the username and to confirm the deletion.

**Usage:**
```bash
bun run admin.ts users delete
```

---

#### `users list`

This command lists all registered users and their roles.

**Usage:**
```bash
bun run admin.ts users list
```

---

### Settings Management (`settings`)

The `settings` command is used to manage application settings.

#### `settings modify`

This command adds a new setting or modifies an existing one.

**Usage:**
```bash
bun run admin.ts settings modify <key> <value>
```

**Example:**
```bash
bun run admin.ts settings modify 'object_detection_enabled' 'true'
```

---

#### `settings list`

This command lists all settings. You can also provide an optional key to view a specific setting.

**Usage:**
```bash
# List all settings
bun run admin.ts settings list

# List a specific setting
bun run admin.ts settings list <key>
```

---

### Secrets Management (`secrets`)

The `secrets` command is used to manage sensitive information like API keys or webhook URLs.

#### `secrets modify`

This command adds a new secret or modifies an existing one.

**Usage:**
```bash
bun run admin.ts secrets modify <key> <value>
```

**Example:**
```bash
bun run admin.ts secrets modify 'webhook_url' 'https://example.com/webhook'
```

---

#### `secrets list`

This command lists all secret keys. For security reasons, it does not display the secret values by default. To view a specific secret's value, you must provide the key.

**Usage:**
```bash
# List all secret keys
bun run admin.ts secrets list

# Show a specific secret's value
bun run admin.ts secrets list <key>
```

---

### Reset Application (`reset`)

The `reset` command will completely reset your Unblink instance by deleting all data, including users, settings, and secrets. This action is irreversible.

**Usage:**
```bash
bun run admin.ts reset
```
You will be asked for confirmation before any data is deleted.
