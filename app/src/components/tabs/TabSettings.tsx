import { FiBell, FiCpu } from 'solid-icons/fi';
import { createSignal, type Component } from 'solid-js';
import Switch from '../Switch';
import { ArkTabs } from '../../ark/ArkTabs';

// Settings state (will be connected to server later)
interface Settings {
  objectDetectionEnabled: boolean;
  webhookUrl: string;
}

const TabSettings: Component<{ onSave?: () => void; isSaving?: boolean }> = (props) => {
  const [settings, setSettings] = createSignal<Settings>({
    objectDetectionEnabled: false,
    webhookUrl: '',
  });

  const updateSetting = <K extends keyof Settings>(key: K, value: Settings[K]) => {
    setSettings((prev) => ({ ...prev, [key]: value }));
  };

  const handleSave = async () => {
    // TODO: Implement server-side save
    console.log('Saving settings:', settings());
    props.onSave?.();
  };

  return (
    <ArkTabs
      items={[
        {
          value: 'machine_learning',
          label: 'Machine Learning',
          icon: <FiCpu size={16} />,
          content: <MachineLearningSettings settings={settings()} updateSetting={updateSetting} />
        },
        {
          value: 'alerts',
          label: 'Alerts',
          icon: <FiBell size={16} />,
          content: <AlertsSettings settings={settings()} updateSetting={updateSetting} />
        }
      ]}
      defaultValue="machine_learning"
    />
  );
};

// Sub-components for each settings section

interface SettingsSectionProps {
  settings: Settings;
  updateSetting: <K extends keyof Settings>(key: K, value: Settings[K]) => void;
}

const MachineLearningSettings: Component<SettingsSectionProps> = (props) => {
  return (
    <div class="p-6 space-y-4">
      <div class="bg-neu-850 border border-neu-800 rounded-lg p-4">
        <Switch
          checked={props.settings.objectDetectionEnabled}
          onCheckedChange={(checked) => props.updateSetting('objectDetectionEnabled', checked)}
          label="Enable Object Detection"
          description="Automatically detect and classify objects in video streams"
        />
      </div>
    </div>
  );
};

const AlertsSettings: Component<SettingsSectionProps> = (props) => {
  return (
    <div class="p-6 space-y-4">
      <div class="bg-neu-850 border border-neu-800 rounded-lg p-4 space-y-4">
        <div>
          <label class="block text-sm font-medium text-neu-300 ">
            Webhook URL
          </label>

          <p class="text-xs text-neu-400 my-1">
            Receive alert notifications via HTTP POST requests
          </p>
          <input
            type="url"
            value={props.settings.webhookUrl}
            onInput={(e) => props.updateSetting('webhookUrl', e.currentTarget.value)}
            placeholder="https://your-domain.com/webhook"
            class="w-full px-3 py-2 rounded-lg bg-neu-900 border border-neu-700 text-white placeholder:text-neu-500 focus:outline-none focus:border-violet-500"
          />
        </div>
      </div>

      {/* Coming soon sections */}
      <div class="bg-neu-850 border border-neu-800 rounded-lg p-4 opacity-50">
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm font-medium text-white">WhatsApp Notifications</div>
            <p class="text-xs text-neu-400 mt-1">Coming soon</p>
          </div>
        </div>
      </div>

      <div class="bg-neu-850 border border-neu-800 rounded-lg p-4 opacity-50">
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm font-medium text-white">Email Notifications</div>
            <p class="text-xs text-neu-400 mt-1">Coming soon</p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default TabSettings;
