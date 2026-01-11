import { Tabs } from '@ark-ui/solid'
import { JSX, Setter } from 'solid-js'

export interface TabItem {
  value: string
  label: string
  content: JSX.Element
  icon?: JSX.Element
}

export const ArkTabs = (props: {
  items: TabItem[]
  defaultValue?: string
  value?: string
  onValueChange?: Setter<string>
}) => {
  return (
    <Tabs.Root
      defaultValue={props.defaultValue || props.items[0]?.value}
      value={props.value}
      onValueChange={(details) => props.onValueChange?.(details.value)}
      class="h-full flex flex-col"
    >
      <Tabs.List class="flex border-b border-neu-800 px-6 relative">
        {props.items.map((item) => (
          <Tabs.Trigger
            value={item.value}
            class="px-4 py-3 text-sm font-medium text-neu-400 hover:text-white transition-colors border-b-2 border-transparent data-[selected]:text-white flex items-center gap-2"
          >
            {item.icon}
            {item.label}
          </Tabs.Trigger>
        ))}
        <Tabs.Indicator class="bg-violet-500" style={{ height: '2px', width: 'var(--width)' }} />
      </Tabs.List>
      {props.items.map((item) => (
        <Tabs.Content value={item.value} class="flex-1 overflow-auto">
          {item.content}
        </Tabs.Content>
      ))}
    </Tabs.Root>
  )
}
