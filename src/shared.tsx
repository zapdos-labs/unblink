import { createSignal } from "solid-js";
import type { ClientToServerMessage, ServerToClientMessage, Subscription, User } from "~/shared";
import { toaster } from "./ark/ArkToast";
import type { Conn } from "~/shared/Conn";

export type Camera = {
    id: string;
    name: string;
    uri: string;
    labels: string[];
    updated_at: string;
    saveToDisk: boolean;
    saveDir: string;
};

export type Tab = {
    type: 'home' | 'search' | 'moments' | 'history' | 'settings';
} | {
    type: 'view';
    medias: {
        stream_id: string;
        file_name?: string;
    }[]
} | {
    type: 'search_result';
    query: string;
}

export const [isAuthenticated, setIsAuthenticated] = createSignal(false);
export const [user, setUser] = createSignal<User>();
export const authorized_as_admin = () => {
    if (settings()['auth_screen_enabled'] !== 'true') return true; // if auth screen is disabled, all users are admins  
    const u = user();
    return u && u.role === 'admin';
}
export const [tab, setTab] = createSignal<Tab>({ type: 'home' });
export const [cameras, setCameras] = createSignal<Camera[]>([]);
export const [camerasLoading, setCamerasLoading] = createSignal(true);
export const [subscription, setSubscription] = createSignal<Subscription>();
export const [conn, setConn] = createSignal<Conn<ClientToServerMessage, ServerToClientMessage>>();
export const [settingsLoaded, setSettingsLoaded] = createSignal(false);
export const [settings, setSettings] = createSignal<Record<string, string>>({});
export const fetchSettings = async () => {
    try {
        const response = await fetch("/settings");
        const data = await response.json();
        const settingsMap: Record<string, string> = {};
        for (const setting of data) {
            settingsMap[setting.key] = setting.value;
        }

        console.log("Fetched settings:", settingsMap);
        setSettings(settingsMap);
        setSettingsLoaded(true);
    } catch (error) {
        console.error("Error fetching settings:", error);
    }
};
export const fetchCameras = async () => {
    setCamerasLoading(true);
    try {
        const response = await fetch('/media');
        if (response.ok) {
            const data = await response.json();
            setCameras(data);
        } else {
            console.error('Failed to fetch media');
            setCameras([]);
        }
    } catch (error) {
        console.error('Error fetching media:', error);
        setCameras([]);
    } finally {
        setCamerasLoading(false);
    }
};

export type Card = {
    created_at: number;
    stream_id: string;
    content: string;
}

export const [agentCards, setAgentCards] = createSignal<Card[]>([]);
export const relevantAgentCards = () => {
    const viewedMedias = () => {
        const t = tab();
        return t.type === 'view' ? t.medias : [];
    }
    const liveStreams = viewedMedias().filter(m => !m.file_name)
    const cards = agentCards();
    // newest first
    const relevant_cards = cards.filter(c => liveStreams.some(media => media.stream_id === c.stream_id)).toSorted((a, b) => b.created_at - a.created_at);

    return relevant_cards;
}