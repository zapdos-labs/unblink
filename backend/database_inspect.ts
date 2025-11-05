import type { MediaUnit } from "~/shared";
import { table_media_units } from "./database";

// For testing
// If run this file directly, try dumping the table
if (require.main === module) {
    const mediaUnits = await table_media_units.query().limit(10).where(`embedding IS NOT NULL`).toArray() as (MediaUnit)[];
    console.log(JSON.stringify(mediaUnits.map(mu => ({ id: mu.id, description: mu.description, embedding: (mu.embedding && mu.embedding.length > 20) ? `${mu.embedding}`.slice(0, 20) + '...' : mu.embedding })), null, 2));
}