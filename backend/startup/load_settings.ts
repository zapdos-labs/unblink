import { table_settings } from "../database";
import { logger } from "../logger";

export async function load_settings() {

    // Load settings into memory
    let SETTINGS: Record<string, string> = {};
    const settings_db = await table_settings.query().toArray();
    for (const setting of settings_db) {
        SETTINGS[setting.key] = setting.value;
    }
    logger.info({ SETTINGS }, "Loaded settings from database");

    const settings = () => SETTINGS;
    return { settings, setSettings: (key: string, value: string) => { SETTINGS[key] = value; } };
}