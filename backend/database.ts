import * as lancedb from "@lancedb/lancedb";
import * as arrow from "apache-arrow";
import type { MediaUnit } from "~/shared";
import { DATABASE_EMBEDDING_DIMENSION, DATABASE_PATH } from "./appdir";
import { onboardMedia, onboardSettings } from "./database_onboarding";
/**
 * Initializes the database and creates the table schema.
 */
export async function initializeDatabase(opts: {
    databasePath: string; embeddingDimension: number

}): Promise<lancedb.Connection> {
    console.log(`Connecting to database at ${opts.databasePath}...`);
    const db = await lancedb.connect(opts.databasePath);
    console.log("Connected to database.");
    const tableNames = await db.tableNames();
    console.log("Existing tables:", tableNames);

    if (!tableNames.includes('media_units')) {
        const schema = new arrow.Schema([
            new arrow.Field('id', new arrow.Utf8()),
            new arrow.Field('media_id', new arrow.Utf8()),
            new arrow.Field('at_time', new arrow.Timestamp(arrow.TimeUnit.MILLISECOND)),
            new arrow.Field('description', new arrow.Utf8(), true),
            new arrow.Field('embedding', new arrow.FixedSizeList(opts.embeddingDimension, new arrow.Field('item', new arrow.Float32(), true)), true),
            new arrow.Field('path', new arrow.Utf8()),
            new arrow.Field('type', new arrow.Utf8()),
        ]);
        await db.createTable({ name: 'media_units', data: [], schema, mode: 'overwrite' });
        console.log("Table 'media_units' created.");
    }

    if (!tableNames.includes('media')) {
        const schema = new arrow.Schema([
            new arrow.Field('id', new arrow.Utf8()),
            new arrow.Field('name', new arrow.Utf8()),
            new arrow.Field('uri', new arrow.Utf8()),
            new arrow.Field('labels', new arrow.List(new arrow.Field('item', new arrow.Utf8())), true),
            new arrow.Field('updated_at', new arrow.Timestamp(arrow.TimeUnit.MILLISECOND)),
            new arrow.Field('saveToDisk', new arrow.Bool(), true),
            new arrow.Field('saveDir', new arrow.Utf8(), true),
        ]);
        await db.createTable({ name: 'media', data: [], schema, mode: 'overwrite' });
        console.log("Table 'media' created.");
        await onboardMedia(db);
    }

    if (!tableNames.includes('settings')) {
        const schema = new arrow.Schema([
            new arrow.Field('key', new arrow.Utf8()),
            new arrow.Field('value', new arrow.Utf8()),
        ]);
        await db.createTable({ name: 'settings', data: [], schema, mode: 'overwrite' });
        console.log("Table 'settings' created.");
        await onboardSettings(db);
    }

    if (!tableNames.includes('secrets')) {
        const schema = new arrow.Schema([
            new arrow.Field('key', new arrow.Utf8()),
            new arrow.Field('value', new arrow.Utf8()),
        ]);
        await db.createTable({ name: 'secrets', data: [], schema, mode: 'overwrite' });
        console.log("Table 'secrets' created.");
    }

    if (!tableNames.includes('sessions')) {
        const schema = new arrow.Schema([
            new arrow.Field('session_id', new arrow.Utf8()),
            new arrow.Field('user_id', new arrow.Utf8()),
            new arrow.Field('created_at', new arrow.Timestamp(arrow.TimeUnit.MILLISECOND)),
            // new arrow.Field('last_activity', new arrow.Timestamp(arrow.TimeUnit.MILLISECOND)),
            new arrow.Field('expires_at', new arrow.Timestamp(arrow.TimeUnit.MILLISECOND)),
        ]);
        await db.createTable({ name: 'sessions', data: [], schema, mode: 'overwrite' });
        console.log("Table 'sessions' created.");
    }

    if (!tableNames.includes('users')) {
        const schema = new arrow.Schema([
            new arrow.Field('id', new arrow.Utf8()),
            new arrow.Field('username', new arrow.Utf8()),
            new arrow.Field('password_hash', new arrow.Utf8()),
            new arrow.Field('role', new arrow.Utf8()),
        ]);
        await db.createTable({ name: 'users', data: [], schema, mode: 'overwrite' });
        console.log("Table 'users' created.");
    }

    return db;
}

export const connection = await initializeDatabase({
    databasePath: DATABASE_PATH,
    embeddingDimension: DATABASE_EMBEDDING_DIMENSION
});

export const table_media_units = await connection.openTable('media_units');
export const table_media = await connection.openTable('media');
export const table_settings = await connection.openTable('settings');
export const table_secrets = await connection.openTable('secrets');
export const table_users = await connection.openTable('users');
export const table_sessions = await connection.openTable('sessions');

let write_queue: {
    type: 'add' | 'update',
    data: MediaUnit | (Partial<MediaUnit> & { id: string })
}[] = []
let write_timeout: NodeJS.Timeout | null = null;

export async function processWriteQueue() {
    // console.log('Processing write queue immediately, length:', write_queue.length);
    const queue = write_queue;
    write_queue = [];
    try {
        const updates = queue.filter(w => w.type === 'update').map(w => w.data as (Partial<MediaUnit> & { id: string }));
        const adds = queue.filter(w => w.type === 'add').map(w => w.data as MediaUnit);
        // console.log(`Processing write queue immediately with ${adds.length} adds and ${updates.length} updates`);
        if (adds.length > 0) {
            await table_media_units.add(adds);
        }
        if (updates.length > 0) {
            await updateMediaUnitBatch(updates);
        }
    } catch (e) {
        console.error('Error processing write queue inner', e, queue.at(0), queue.at(1));
    }
}

export function processWriteQueue_lazy() {
    if (write_queue.length === 0) return;
    if (write_timeout) clearTimeout(write_timeout);
    if (write_queue.length > 20) {
        // console.log('more than 20 items, processing immediately, length:', write_queue.length);
        // If more than 20 items, process immediately
        processWriteQueue();
    } else {
        write_timeout = setTimeout(() => {
            // console.log('5 seconds passed, processing write queue, length:', write_queue.length);
            processWriteQueue();
        }, 5000);
    }
}

export function addMediaUnit(mediaUnit: MediaUnit) {
    try {
        const addable = {
            ...mediaUnit,
            at_time: new Date(mediaUnit.at_time),
            description: mediaUnit.description ?? null,
            embedding: mediaUnit.embedding ? mediaUnit.embedding : null,
        }

        write_queue.push({
            type: 'add',
            data: addable
        });
        processWriteQueue_lazy();
    } catch (e) {
        console.error('Error adding media unit outer', e);
    }
}


export function partialMediaUnitToUpdate(mediaUnit: Partial<MediaUnit> & { id: string }, coalesce?: Record<string, any>) {
    const update: Record<string, any> = {};
    // Only include defined fields
    for (const key in mediaUnit) {
        if (mediaUnit[key as keyof Partial<MediaUnit>] !== undefined) {
            update[key] = mediaUnit[key as keyof Partial<MediaUnit>];
        }
    }

    // Coalesce fields
    for (const key in coalesce) {
        if (update[key] === undefined || update[key] === null) {
            update[key] = coalesce[key];
        }
    }

    return update;
}


export function updateMediaUnit(mediaUnit: Partial<MediaUnit> & { id: string }) {
    try {
        write_queue.push({
            type: 'update',
            data: mediaUnit
        });
        processWriteQueue_lazy();
    }
    catch (e) {
        console.error('Error updating media unit outer', e);
    }
}
export async function updateMediaUnitBatch(mediaUnits: (Partial<MediaUnit> & { id: string })[]): Promise<void> {
    try {
        // Merge updates by id first
        const mergedUpdates: Record<string, Partial<MediaUnit> & { id: string }> = {};
        for (const mu of mediaUnits) {
            const id = mu.id;
            if (!mergedUpdates[id]) mergedUpdates[id] = { id };
            Object.assign(mergedUpdates[id], mu);
        }

        // NOW call partialMediaUnitToUpdate on the merged updates
        const rowUpdates = Object.values(mergedUpdates).map(merged =>
            partialMediaUnitToUpdate(merged, { embedding: null })
        );

        const result = await table_media_units.mergeInsert("id")
            .whenMatchedUpdateAll()
            .execute(rowUpdates);

        // console.log('updated media units:', result)
    } catch (error) {
        console.error("Error updating media unit batch:", error);
    }
}

/**
 * Searches for media units by embedding similarity.
 */
export async function searchMediaUnitsByEmbedding(queryEmbedding: number[]): Promise<(MediaUnit & { _distance: number })[] | null> {
    try {
        const results = table_media_units.search(queryEmbedding).where(`description IS NOT NULL`).limit(20);
        const resultArray = await results.toArray();
        return resultArray;
    } catch (error) {
        console.error("Error searching media units by embedding:", error);
        return null;
    }
}


export async function getMediaUnitById(id: string): Promise<MediaUnit | null> {
    try {
        const results = await table_media_units.query().where(`id = '${id}'`).limit(1).toArray();
        if (results.length === 0) return null;
        return results[0];
    } catch (error) {
        console.error("Error retrieving media unit by id:", error);
        return null;
    }
}