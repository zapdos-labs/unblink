import { Match, Switch, createSignal, type Component } from 'solid-js';
import SideBar from './SideBar';
import VideoTile from './VideoTile';
import TabLayout from './TabLayout';
import TabSettings from './tabs/TabSettings';
import TabHome from './tabs/TabHome';
import { tab, type Tab } from '../shared';

const ComingSoon = () => (
  <div class="flex justify-center items-center h-full">
    <p class="text-neu-500">Coming soon</p>
  </div>
);

const Main: Component = () => {
  const [isSavingSettings, setIsSavingSettings] = createSignal(false);

  const handleSaveSettings = async () => {
    setIsSavingSettings(true);
    // TODO: Implement server-side save
    console.log('Saving settings...');
    await new Promise((resolve) => setTimeout(resolve, 500));
    setIsSavingSettings(false);
  };

  return (
    <div class="flex gap-2 h-screen bg-neutral-950">
      <SideBar />

      <div class="flex-1 h-screen overflow-hidden">
        <Switch fallback={
          <TabLayout title="Home">
            <div class="flex justify-center items-center h-full">
              <p class="text-neu-500">Welcome to Unblink Dashboard</p>
            </div>
          </TabLayout>
        }>
          <Match when={tab().type === 'view'}>
            {(() => {
              const t = tab() as Extract<Tab, { type: 'view' }>;
              return (
                <TabLayout title={t.name || 'Stream'}>
                  <VideoTile nodeId={t.nodeId} serviceId={t.serviceId} name={t.name} />
                </TabLayout>
              );
            })()}
          </Match>

          <Match when={tab().type === 'search'}>
            <TabLayout title="Search">
              <ComingSoon />
            </TabLayout>
          </Match>

          <Match when={tab().type === 'moments'}>
            <TabLayout title="Moments">
              <ComingSoon />
            </TabLayout>
          </Match>

          <Match when={tab().type === 'agents'}>
            <TabLayout title="Agents">
              <ComingSoon />
            </TabLayout>
          </Match>

          <Match when={tab().type === 'settings'}>
            <TabLayout
              title="Settings"
              headerAction={
                <button
                  onClick={handleSaveSettings}
                  disabled={isSavingSettings()}
                  class="drop-shadow-2xl px-4 py-2 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 flex items-center justify-center disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  {isSavingSettings() ? 'Saving...' : 'Save Settings'}
                </button>
              }
            >
              <TabSettings onSave={handleSaveSettings} isSaving={isSavingSettings()} />
            </TabLayout>
          </Match>

          <Match when={tab().type === 'home'}>
            <TabLayout title="Home">
              <TabHome />
            </TabLayout>
          </Match>
        </Switch>
      </div>
    </div>
  );
};

export default Main;
