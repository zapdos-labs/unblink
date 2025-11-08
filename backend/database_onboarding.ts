import { v4 as uuidv4 } from 'uuid';
import * as lancedb from "@lancedb/lancedb";
import { logger } from './logger';

export async function onboardSettings(connection: lancedb.Connection) {
    logger.info("Onboarding settings...");
    const settingsTable = await connection.openTable('settings');
    await settingsTable.add([{ key: 'object_detection_enabled', value: 'true' }]);
    logger.info("Default settings added.");
}

export async function onboardMedia(connection: lancedb.Connection) {
    logger.info("Onboarding media from predefined list...");

    const table_media = await connection.openTable('media');

    const mediaList = [
        {
            name: "Building Top",
            uri: "rtsp://www.cactus.tv:1554/cam58",
            labels: ["Urban"]
        },
        {
            name: "Panama Port",
            uri: "http://200.46.196.243/axis-cgi/media.cgi?camera=1&videoframeskipmode=empty&videozprofile=classic&resolution=1280x720&audiodeviceid=0&audioinputid=0&audiocodec=aac&audiosamplerate=16000&audiobitrate=32000&timestamp=0&videocodec=h264&container=mp4",
            labels: ["Transportation Hub"]
        },
        {
            name: "Parking Lot",
            uri: "http://83.48.75.113:8320/axis-cgi/mjpg/video.cgi",
            labels: ["Urban"]
        },
    ];

    const newMediaEntries = mediaList.map(media => ({
        id: uuidv4(),
        name: media.name,
        uri: media.uri,
        labels: media.labels,
        updated_at: new Date(),
    }));

    await table_media.add(newMediaEntries);
    console.log(`${newMediaEntries.length} media entries onboarded successfully.`);
}
