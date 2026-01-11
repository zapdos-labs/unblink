
import { Switch } from '@ark-ui/solid';

export default function ArkSwitch(props: {
    checked: () => boolean,
    onCheckedChange: (details: { checked: boolean }) => void,
    label: string
}) {
    return <Switch.Root
        checked={props.checked()}
        onCheckedChange={props.onCheckedChange}
        class="flex items-center "
    >
        <Switch.Control class="relative inline-flex h-6 w-11 items-center rounded-full border-2 border-transparent transition-colors focus:outline-none data-[state=checked]:bg-violet-600 data-[state=unchecked]:bg-neu-700 cursor-pointer">
            <Switch.Thumb class="inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out data-[state=checked]:translate-x-5 data-[state=unchecked]:translate-x-0" />
        </Switch.Control>
        <Switch.Label class="ml-3 text-sm font-medium text-neu-300">{props.label}</Switch.Label>
        <Switch.HiddenInput />
    </Switch.Root>
}
