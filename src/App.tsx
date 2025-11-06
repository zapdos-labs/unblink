
import { createEffect, onMount, type ValidComponent } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import ArkToast from './ark/ArkToast';
import HomeContent from './content/HomeContent';
import MomentsContent from './content/MomentsContent';
import { conn, fetchCameras, fetchSettings, setAgentCards, setConn, subscription, tab } from './shared';
import SideBar from './SideBar';
import { connectWebSocket, newMessage } from './video/connection';
import ViewContent from './ViewContent';
import HistoryContent from './content/HistoryContent';
import SettingsContent from './content/SettingsContent';
import SearchContent from './content/SearchContent';

export default function App() {
    onMount(() => {
        const conn = connectWebSocket();
        setConn(conn);
        fetchSettings();
        fetchCameras();
    })

    createEffect(() => {
        const m = newMessage();
        if (!m) return;

        if (m.type === 'frame_description') {
            // console.log('Received description for stream', m.stream_id, ':', m.description);
            setAgentCards(prev => {
                return [...prev, { created_at: Date.now(), stream_id: m.stream_id, content: m.description }].slice(-200);
            });
        }
    })


    createEffect(() => {
        const c = conn();
        const _subscription = subscription();
        if (!c) return;
        c.send({ type: 'set_subscription', subscription: _subscription });

    })

    const components = (): Record<string, ValidComponent> => {
        return {
            'home': HomeContent,
            'moments': MomentsContent,
            'view': ViewContent,
            'history': HistoryContent,
            'search': SearchContent,
            'settings': SettingsContent,
        }

    }
    const component = () => components()[tab().type]

    return <div class="h-screen flex items-start bg-neu-925 text-white space-x-2">
        <ArkToast />
        <SideBar />
        <div class="flex-1">
            <Dynamic component={component()} />
        </div>

    </div>;
}