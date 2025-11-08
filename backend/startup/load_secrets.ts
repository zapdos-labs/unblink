import { generateSecret } from "../auth";
import { table_secrets } from "../database";
import { logger } from "../logger";

export async function load_secrets() {
    // Load secrets into memory
    const SECRETS: Record<string, string> = {};
    const secrets_db = await table_secrets.query().toArray();
    for (const secret of secrets_db) {
        SECRETS[secret.key] = secret.value;
    }
    logger.info("Loaded secrets from database");

    if (!SECRETS['jwt_signing_key']) {
        const new_key = generateSecret(64);
        await table_secrets.add([{ key: 'jwt_signing_key', value: new_key }]);
        SECRETS['jwt_signing_key'] = new_key;
        logger.info("Generated new JWT signing key");
    }


    const secrets = () => SECRETS;
    return { secrets };
}