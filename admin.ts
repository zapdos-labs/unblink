import { table_users, table_settings, table_secrets } from './backend/database';
import { randomUUID } from 'crypto';
import { createInterface } from 'node:readline/promises';
import { hashPassword } from './backend/auth';

const rl = createInterface({
    input: process.stdin,
    output: process.stdout,
});

// --- Utility Functions ---

const helpText = {
    users: {
        description: 'Manage users',
        usage: 'bun run admin.ts users <subcommand>',
        subcommands: `
      add                 Add a new user
      update              Update an existing user's password
      delete              Delete a user
      list                List all users`
    },
    settings: {
        description: 'Manage settings',
        usage: 'bun run admin.ts settings <subcommand>',
        subcommands: `
      modify <key> <value>  Add or modify a setting
      list [<key>]          List all settings or a specific key`
    },
    secrets: {
        description: 'Manage secrets',
        usage: 'bun run admin.ts secrets <subcommand>',
        subcommands: `
      modify <key> <value>  Add or modify a secret
      list [<key>]          List all secret keys or a specific secret's value`
    }
};

type HelpTextKeys = 'users' | 'settings' | 'secrets';

function showHelp(section?: HelpTextKeys) {
    if (section) {
        const config = helpText[section];
        console.log(`\nUsage: ${config.usage}\n\n${config.description}.\n\nSubcommands:${config.subcommands}\n`);
    } else {
        console.log('\nUsage: bun run admin.ts <command>\n\nCommands:\n');
        for (const [key, value] of Object.entries(helpText)) {
            console.log(`  ${key.padEnd(22)}${value.description}`);
        }
        console.log(`  ${'help, --help'.padEnd(22)}Show this help message\n`);
    }
}


// --- User Management Functions ---

async function addUser() {
    const username = (await rl.question('Enter username: ')).trim();
    if (!username) throw new Error('Username cannot be empty.');

    const existingUser = await table_users.query().where(`username = '${username}'`).limit(1).toArray();
    if (existingUser.length > 0) throw new Error(`User '${username}' already exists.`);

    const password = (await rl.question('Enter password: ')).trim();
    if (!password) throw new Error('Password cannot be empty.');

    let role = '';
    const validRoles = ['admin', 'viewer'];
    while (true) {
        role = (await rl.question('Enter role (admin | viewer): ')).trim();
        if (validRoles.includes(role)) break;
        console.log('Invalid role. Please enter "admin" or "viewer".');
    }

    const argonHash = await hashPassword(password);
    const id = randomUUID();
    await table_users.add([{ id, username, password_hash: argonHash, role }]);
    console.log(`New user '${username}' created successfully with role '${role}'.`);
}

async function updateUser() {
    const username = (await rl.question('Enter username: ')).trim();
    if (!username) throw new Error('Username cannot be empty.');

    const existingUser = await table_users.query().where(`username = '${username}'`).limit(1).toArray();
    if (existingUser.length === 0) throw new Error(`User '${username}' not found.`);

    const password = (await rl.question('Enter new password: ')).trim();
    if (!password) throw new Error('Password cannot be empty.');

    const argonHash = await hashPassword(password);
    await table_users.mergeInsert("username")
        .whenMatchedUpdateAll()
        .execute([{ username, password_hash: argonHash }]);
    console.log(`Password for user '${username}' has been updated successfully.`);
}

async function deleteUser() {
    const username = (await rl.question('Enter username to delete: ')).trim();
    if (!username) throw new Error('Username cannot be empty.');

    const existingUser = await table_users.query().where(`username = '${username}'`).limit(1).toArray();
    if (existingUser.length === 0) throw new Error(`User '${username}' not found.`);

    const confirmation = (await rl.question(`Are you sure you want to delete user '${username}'? (yes/no): `)).trim().toLowerCase();
    if (confirmation !== 'yes') {
        console.log('Deletion cancelled.');
        return;
    }

    await table_users.delete(`username = '${username}'`);
    console.log(`User '${username}' has been deleted.`);
}

async function listUsers() {
    const users = await table_users.query().toArray();
    if (users.length === 0) {
        console.log("No users found.");
        return;
    }
    console.log("Users:");
    users.forEach(user => {
        // @ts-ignore
        console.log(`  - Username: ${user.username}, Role: ${user.role}`);
    });
}

// --- Settings Management Functions ---

async function listSettings() {
    const key = process.argv[4];
    if (key) {
        const setting = await table_settings.query().where(`key = '${key}'`).limit(1).toArray();
        if (setting.length === 0) {
            console.log(`Setting with key '${key}' not found.`);
        } else {
            // @ts-ignore
            console.log(`${setting[0].key}: ${setting[0].value}`);
        }
    } else {
        const settings = await table_settings.query().toArray();
        if (settings.length === 0) {
            console.log("No settings found.");
            return;
        }
        console.log("Settings:");
        settings.forEach(setting => {
            // @ts-ignore
            console.log(`  - ${setting.key}: ${setting.value}`);
        });
    }
}

async function modifySetting() {
    const key = process.argv[4];
    const value = process.argv[5];

    if (!key || value === undefined) {
        throw new Error("Usage: settings modify <key> <value>");
    }

    await table_settings.mergeInsert("key")
        .whenMatchedUpdateAll()
        .whenNotMatchedInsertAll()
        .execute([{ key, value: value.toString() }]);

    console.log(`Setting '${key}' has been set to '${value}'.`);
}

// --- Secrets Management Functions ---

async function listSecrets() {
    const key = process.argv[4];
    if (key) {
        const secret = await table_secrets.query().where(`key = '${key}'`).limit(1).toArray();
        if (secret.length === 0) {
            console.log(`Secret with key '${key}' not found.`);
        } else {
            // @ts-ignore
            console.log(`${secret[0].key}: ${secret[0].value}`);
        }
    } else {
        const secrets = await table_secrets.query().toArray();
        if (secrets.length === 0) {
            console.log("No secrets found.");
            return;
        }
        console.log("Secret keys:");
        secrets.forEach(secret => {
            // @ts-ignore
            console.log(`  - ${secret.key}`);
        });
    }
}

async function modifySecret() {
    const key = process.argv[4];
    const value = process.argv[5];

    if (!key || value === undefined) {
        throw new Error("Usage: secrets modify <key> <value>");
    }

    await table_secrets.mergeInsert("key")
        .whenMatchedUpdateAll()
        .whenNotMatchedInsertAll()
        .execute([{ key, value: value.toString() }]);

    console.log(`Secret '${key}' has been set.`);
}

// --- Command Handlers ---

async function handleUsersCommand() {
    const subcommand = process.argv[3];
    switch (subcommand) {
        case 'add':
            await addUser();
            break;
        case 'update':
            await updateUser();
            break;
        case 'delete':
            await deleteUser();
            break;
        case 'list':
            await listUsers();
            break;
        case 'help':
        case undefined:
            showHelp('users');
            break;
        default:
            console.log(`Unknown users command: '${subcommand}'\n`);
            showHelp('users');
            process.exitCode = 1;
    }
}

async function handleSettingsCommand() {
    const subcommand = process.argv[3];
    switch (subcommand) {
        case 'modify':
            await modifySetting();
            break;
        case 'list':
            await listSettings();
            break;
        case 'help':
        case undefined:
            showHelp('settings');
            break;
        default:
            console.log(`Unknown settings command: '${subcommand}'\n`);
            showHelp('settings');
            process.exitCode = 1;
    }
}

async function handleSecretsCommand() {
    const subcommand = process.argv[3];
    switch (subcommand) {
        case 'modify':
            await modifySecret();
            break;
        case 'list':
            await listSecrets();
            break;
        case 'help':
        case undefined:
            showHelp('secrets');
            break;
        default:
            console.log(`Unknown secrets command: '${subcommand}'\n`);
            showHelp('secrets');
            process.exitCode = 1;
    }
}

// --- Main Execution ---

async function main() {
    try {
        const command = process.argv[2];

        switch (command) {
            case 'users':
                await handleUsersCommand();
                break;
            case 'settings':
                await handleSettingsCommand();
                break;
            case 'secrets':
                await handleSecretsCommand();
                break;
            case 'help':
            case '--help':
            case undefined:
                showHelp();
                break;
            default:
                console.log(`Unknown command: ${command}\n`);
                showHelp();
                process.exitCode = 1;
        }
    } catch (error) {
        if (error instanceof Error) {
            console.error(error.message);
        } else {
            console.error(error);
        }
        process.exitCode = 1;
    } finally {
        rl.close();
    }
}

main();