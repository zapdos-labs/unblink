import path from "path";
import fs from 'fs/promises';

async function ensureDirExists(dir: string) {
    try {
        await fs.mkdir(dir, { recursive: true });
    } catch (e) {
        console.error(`Error creating directory ${dir}:`, e);
    }
}

export const appdir = () => process.env.APPDATA || (process.platform == 'darwin' ? process.env.HOME + '/Library/Preferences' : process.env.HOME + "/.local/share")
export const APP_NAME = "unblink";
export const RUNTIME_DIR = path.join(appdir(), APP_NAME);
export const MODELS_DIR = path.join(RUNTIME_DIR, 'models');


export const DATABASE_PATH = path.join(RUNTIME_DIR, 'database');
export const DATABASE_EMBEDDING_DIMENSION = 2048;
export const FILES_DIR = path.join(RUNTIME_DIR, 'files');
export const FRAMES_DIR = path.join(FILES_DIR, 'frames');
export const RECORDINGS_DIR = path.join(FILES_DIR, 'recordings');

// Create directories if they don't exist
await ensureDirExists(RUNTIME_DIR);
await ensureDirExists(MODELS_DIR);
await ensureDirExists(FILES_DIR);
await ensureDirExists(DATABASE_PATH);