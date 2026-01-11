import { Switch as ArkSwitch } from '@ark-ui/solid';

interface SwitchProps {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  label: string;
  description?: string;
}

export default function Switch(props: SwitchProps) {
  return (
    <ArkSwitch.Root
      checked={props.checked}
      onCheckedChange={(details) => props.onCheckedChange(details.checked)}
      class="flex items-center justify-between w-full"
    >
      <div class="flex-1">
        <ArkSwitch.Label class="text-sm font-medium text-white cursor-pointer">
          {props.label}
        </ArkSwitch.Label>
        {props.description && (
          <p class="text-xs text-neu-400 mt-1">{props.description}</p>
        )}
      </div>
      <ArkSwitch.Control class="relative inline-flex h-6 w-11 items-center rounded-full border-2 border-transparent transition-colors focus:outline-none data-[state=checked]:bg-violet-600 data-[state=unchecked]:bg-neu-700 cursor-pointer flex-shrink-0">
        <ArkSwitch.Thumb class="inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out data-[state=checked]:translate-x-5 data-[state=unchecked]:translate-x-0" />
      </ArkSwitch.Control>
      <ArkSwitch.HiddenInput />
    </ArkSwitch.Root>
  );
}
