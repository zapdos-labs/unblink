import { createResource } from "solid-js";
import LayoutContent from "./LayoutContent";

async function fetchRecordings() {
    const response = await fetch("/recordings");
    return response.json();
}

export default function HistoryContent() {
    const [recordings] = createResource(fetchRecordings);

    return <LayoutContent title="History">
        <pre>{JSON.stringify(recordings(), null, 2)}</pre>
    </LayoutContent>
}